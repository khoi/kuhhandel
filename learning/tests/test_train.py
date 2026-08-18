from __future__ import annotations

import contextlib
import io
import tempfile
import unittest
from pathlib import Path

import torch

from kuhhandel_learning.train import Rollout, RolloutWorker, load_model, parser, policy_gradient, train


class TrainerTests(unittest.TestCase):
    def test_policy_gradient_uses_baseline(self) -> None:
        weights = torch.zeros((1, 2), dtype=torch.float64)
        rollout = Rollout(1, 0.5, 0, 1, [], [[3, 5]], [[1, 2]])
        gradient = policy_gradient(rollout, 0.5, weights)
        self.assertTrue(torch.equal(gradient, torch.tensor([[2.5, 4]], dtype=torch.float64)))

    def test_argument_ranges(self) -> None:
        for arguments in (["--steps", "0"], ["--players", "2"], ["--baseline-rate", "2"]):
            with contextlib.redirect_stderr(io.StringIO()):
                with self.assertRaises(SystemExit):
                    parser().parse_args(arguments)

    def test_worker_reports_shape_and_equal_policy_share(self) -> None:
        root = Path(__file__).resolve().parents[2]
        with RolloutWorker(root) as worker:
            decisions, features = worker.shape()
            weights = torch.zeros((decisions, features), dtype=torch.float64)
            rollout = worker.rollout(weights, 3, 2, 801, False)
        self.assertEqual(rollout.games, 6)
        self.assertAlmostEqual(rollout.mean_reward, 1 / 3)

    def test_training_smoke(self) -> None:
        root = Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as directory:
            arguments = parser().parse_args(
                [
                    "--root",
                    str(root),
                    "--steps",
                    "1",
                    "--batch-seeds",
                    "1",
                    "--eval-seeds",
                    "1",
                    "--held-out-seeds",
                    "1",
                    "--checkpoint",
                    str(Path(directory) / "best.pt"),
                    "--export",
                    str(Path(directory) / "best.json"),
                ]
            )
            result = train(arguments)
            self.assertEqual(result.games, 3)
            self.assertEqual(load_model(arguments.checkpoint, 3).shape, (5, 16))
            self.assertEqual(load_model(arguments.export, 3).shape, (5, 16))
            with self.assertRaises(RuntimeError):
                load_model(arguments.export, 5)


if __name__ == "__main__":
    unittest.main()
