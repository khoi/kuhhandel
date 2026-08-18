from __future__ import annotations

import argparse
import json
import subprocess
from dataclasses import dataclass
from pathlib import Path
from types import TracebackType
from typing import IO, Any

import torch


@dataclass(frozen=True)
class Rollout:
    games: int
    mean_reward: float
    standard_error: float
    mean_decisions: float
    mean_deviations: list[float]
    reward_gradient: list[list[float]]
    mean_gradient: list[list[float]]

    @classmethod
    def from_payload(cls, payload: dict[str, Any]) -> Rollout:
        error = payload.get("error")
        if error:
            raise RuntimeError(str(error))
        return cls(
            games=int(payload["games"]),
            mean_reward=float(payload["mean_reward"]),
            standard_error=float(payload.get("standard_error", 0)),
            mean_decisions=float(payload.get("mean_decisions", 0)),
            mean_deviations=payload.get("mean_deviations", []),
            reward_gradient=payload.get("reward_gradient", []),
            mean_gradient=payload.get("mean_gradient", []),
        )


class RolloutWorker:
    def __init__(self, root: Path, go: str = "go") -> None:
        self._process = subprocess.Popen(
            [go, "run", "./cmd/kuhhandel-rollout"],
            cwd=root,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            bufsize=1,
        )

    def __enter__(self) -> RolloutWorker:
        return self

    def __exit__(
        self,
        exception_type: type[BaseException] | None,
        exception: BaseException | None,
        traceback: TracebackType | None,
    ) -> None:
        self.close()

    def shape(self) -> tuple[int, int]:
        payload = self._request({"kind": "shape"})
        return int(payload["decisions"]), int(payload["features"])

    def rollout(
        self,
        weights: torch.Tensor,
        players: int,
        seeds: int,
        seed: int,
        sample: bool,
        opponents: list[torch.Tensor] | None = None,
    ) -> Rollout:
        payload = self._request(
            {
                "kind": "rollout",
                "players": players,
                "seeds": seeds,
                "seed": seed,
                "sample": sample,
                "weights": weights.detach().cpu().tolist(),
                "opponents": [opponent.detach().cpu().tolist() for opponent in opponents or []],
            }
        )
        return Rollout.from_payload(payload)

    def close(self) -> None:
        if self._process.stdin is not None and not self._process.stdin.closed:
            self._process.stdin.close()
        if self._process.poll() is None:
            try:
                self._process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                self._process.terminate()
                self._process.wait(timeout=5)
        if self._process.stdout is not None:
            self._process.stdout.close()
        if self._process.stderr is not None:
            self._process.stderr.close()

    def _request(self, request: dict[str, Any]) -> dict[str, Any]:
        stdin = self._stdin()
        stdout = self._stdout()
        stdin.write(json.dumps(request, separators=(",", ":")) + "\n")
        stdin.flush()
        line = stdout.readline()
        if line == "":
            stderr = self._process.stderr.read() if self._process.stderr is not None else ""
            raise RuntimeError(stderr.strip() or "rollout worker stopped")
        payload = json.loads(line)
        if not isinstance(payload, dict):
            raise RuntimeError("rollout worker returned a non-object")
        error = payload.get("error")
        if error:
            raise RuntimeError(str(error))
        return payload

    def _stdin(self) -> IO[str]:
        if self._process.stdin is None:
            raise RuntimeError("rollout worker has no input")
        return self._process.stdin

    def _stdout(self) -> IO[str]:
        if self._process.stdout is None:
            raise RuntimeError("rollout worker has no output")
        return self._process.stdout


def policy_gradient(rollout: Rollout, baseline: float, weights: torch.Tensor) -> torch.Tensor:
    reward = torch.tensor(rollout.reward_gradient, dtype=weights.dtype, device=weights.device)
    mean = torch.tensor(rollout.mean_gradient, dtype=weights.dtype, device=weights.device)
    if reward.shape != weights.shape or mean.shape != weights.shape:
        raise RuntimeError("rollout gradient shape does not match the model")
    return reward - baseline * mean


def reward_order(candidate: Rollout, best: Rollout) -> int:
    return int(candidate.mean_reward > best.mean_reward) - int(candidate.mean_reward < best.mean_reward)


def train(arguments: argparse.Namespace) -> Rollout:
    root = repository_root(arguments.root)
    with RolloutWorker(root, arguments.go) as worker:
        decisions, features = worker.shape()
        shape = (decisions, features)
        weights = torch.nn.Parameter(initial_weights(arguments.initial, arguments.players, shape))
        if arguments.freeze_features > features:
            raise RuntimeError(f"cannot freeze {arguments.freeze_features} of {features} features")
        opponents = opponent_pool(
            arguments.opponent,
            arguments.players,
            shape,
            not arguments.exclude_guide,
        )
        optimizer = torch.optim.Adam([weights], lr=arguments.learning_rate)
        baseline = 1 / arguments.players
        validation_seed = arguments.seed + arguments.steps * arguments.batch_seeds
        held_out_seed = validation_seed + arguments.eval_seeds
        best_weights = weights.detach().clone()
        best_step = 0
        best = worker.rollout(best_weights, arguments.players, arguments.eval_seeds, validation_seed, False, opponents)
        print_result("checkpoint 0", best)
        for step in range(1, arguments.steps + 1):
            training_seed = arguments.seed + (step - 1) * arguments.batch_seeds
            rollout = worker.rollout(weights, arguments.players, arguments.batch_seeds, training_seed, True, opponents)
            gradient = policy_gradient(rollout, baseline, weights)
            gradient[:, : arguments.freeze_features] = 0
            optimizer.zero_grad(set_to_none=True)
            weights.grad = -gradient
            torch.nn.utils.clip_grad_norm_([weights], arguments.max_gradient)
            optimizer.step()
            baseline += arguments.baseline_rate * (rollout.mean_reward - baseline)
            if step % arguments.eval_every != 0 and step != arguments.steps:
                continue
            evaluated = worker.rollout(
                weights,
                arguments.players,
                arguments.eval_seeds,
                validation_seed,
                False,
                opponents,
            )
            print_result(f"checkpoint {step}", evaluated)
            order = reward_order(evaluated, best)
            if order > 0:
                best = evaluated
                best_weights = weights.detach().clone()
                best_step = step
            elif order < 0:
                with torch.no_grad():
                    weights.copy_(best_weights)
                optimizer.state.clear()
                baseline = best.mean_reward
        held_out = worker.rollout(
            best_weights,
            arguments.players,
            arguments.held_out_seeds,
            held_out_seed,
            False,
            opponents,
        )
        print_result("held-out", held_out)
        save_checkpoint(arguments.checkpoint, best_weights, arguments.players, best_step, best)
        if arguments.export is not None:
            save_model(arguments.export, arguments.players, best_weights)
        return held_out


def evaluate(arguments: argparse.Namespace) -> Rollout:
    root = repository_root(arguments.root)
    with RolloutWorker(root, arguments.go) as worker:
        decisions, features = worker.shape()
        shape = (decisions, features)
        weights = fit_model(load_model(arguments.evaluate, arguments.players), shape)
        opponents = opponent_pool(
            arguments.opponent,
            arguments.players,
            shape,
            not arguments.exclude_guide,
        )
        result = worker.rollout(weights, arguments.players, arguments.held_out_seeds, arguments.seed, False, opponents)
    print_result("evaluation", result)
    return result


def repository_root(path: Path) -> Path:
    root = path.resolve()
    if not (root / "go.mod").is_file():
        raise RuntimeError(f"{root} is not the Kuhhandel repository")
    return root


def save_checkpoint(
    path: Path,
    weights: torch.Tensor,
    players: int,
    step: int,
    validation: Rollout,
) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    torch.save(
        {
            "weights": weights.detach(),
            "players": players,
            "step": step,
            "validation_reward": validation.mean_reward,
        },
        path,
    )


def save_model(path: Path, players: int, weights: torch.Tensor) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    payload = {"players": players, "weights": weights.detach().cpu().tolist()}
    path.write_text(json.dumps(payload, separators=(",", ":")) + "\n", encoding="utf-8")


def load_model(path: Path, players: int) -> torch.Tensor:
    if path.suffix == ".pt":
        checkpoint = torch.load(path, map_location="cpu", weights_only=True)
        if checkpoint.get("players") != players:
            raise RuntimeError(f"model does not support {players} players")
        value = checkpoint.get("weights")
        if not isinstance(value, torch.Tensor):
            raise RuntimeError("checkpoint has no weights")
        return value.to(dtype=torch.float64)
    payload = json.loads(path.read_text(encoding="utf-8"))
    if not isinstance(payload, dict) or payload.get("players") != players:
        raise RuntimeError(f"model does not support {players} players")
    return torch.tensor(payload.get("weights"), dtype=torch.float64)


def opponent_pool(
    paths: list[Path],
    players: int,
    shape: tuple[int, int],
    include_guide: bool,
) -> list[torch.Tensor]:
    opponents = [torch.zeros(shape, dtype=torch.float64)] if include_guide else []
    for path in paths:
        opponents.append(fit_model(load_model(path, players), shape))
    if not opponents:
        raise RuntimeError("opponent pool is empty")
    return opponents


def initial_weights(path: Path | None, players: int, shape: tuple[int, int]) -> torch.Tensor:
    if path is None:
        return torch.zeros(shape, dtype=torch.float64)
    return fit_model(load_model(path, players), shape)


def fit_model(weights: torch.Tensor, shape: tuple[int, int]) -> torch.Tensor:
    widths = {shape[1] // 2, shape[1]}
    if weights.ndim != 2 or weights.shape[0] != shape[0] or weights.shape[1] not in widths:
        raise RuntimeError(f"model shape {tuple(weights.shape)} does not fit {shape}")
    if weights.shape[1] == shape[1]:
        return weights
    missing = torch.zeros((shape[0], shape[1] - weights.shape[1]), dtype=weights.dtype)
    return torch.cat((weights, missing), dim=1)


def print_result(label: str, rollout: Rollout) -> None:
    deviations = ",".join(f"{count:.2f}" for count in rollout.mean_deviations)
    reward = f"{rollout.mean_reward * 100:.1f}% +/- {rollout.standard_error * 100:.1f}%"
    print(f"{label}: {reward} deviations [{deviations}]", flush=True)


def parser() -> argparse.ArgumentParser:
    result = argparse.ArgumentParser(description="Train a Kuhhandel best response with PyTorch")
    result.add_argument("--players", type=int, choices=range(3, 6), default=3)
    result.add_argument("--steps", type=positive_int, default=100)
    result.add_argument("--batch-seeds", type=positive_int, default=32)
    result.add_argument("--eval-every", type=positive_int, default=10)
    result.add_argument("--eval-seeds", type=positive_int, default=200)
    result.add_argument("--held-out-seeds", type=positive_int, default=1000)
    result.add_argument("--seed", type=nonnegative_int, default=7_000_000)
    result.add_argument("--learning-rate", type=positive_float, default=0.005)
    result.add_argument("--baseline-rate", type=unit_float, default=0.02)
    result.add_argument("--max-gradient", type=positive_float, default=5)
    result.add_argument("--freeze-features", type=nonnegative_int, default=0)
    result.add_argument("--checkpoint", type=Path, default=Path("learning/checkpoints/best.pt"))
    result.add_argument("--export", type=Path)
    result.add_argument("--evaluate", type=Path)
    result.add_argument("--initial", type=Path)
    result.add_argument("--opponent", type=Path, action="append", default=[])
    result.add_argument("--exclude-guide", action="store_true")
    result.add_argument("--root", type=Path, default=Path.cwd())
    result.add_argument("--go", default="go")
    return result


def positive_int(value: str) -> int:
    parsed = int(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be positive")
    return parsed


def nonnegative_int(value: str) -> int:
    parsed = int(value)
    if parsed < 0:
        raise argparse.ArgumentTypeError("must not be negative")
    return parsed


def positive_float(value: str) -> float:
    parsed = float(value)
    if parsed <= 0:
        raise argparse.ArgumentTypeError("must be positive")
    return parsed


def unit_float(value: str) -> float:
    parsed = float(value)
    if parsed <= 0 or parsed > 1:
        raise argparse.ArgumentTypeError("must be greater than zero and at most one")
    return parsed


def main() -> None:
    arguments = parser().parse_args()
    if arguments.evaluate is None:
        train(arguments)
    else:
        evaluate(arguments)


if __name__ == "__main__":
    main()
