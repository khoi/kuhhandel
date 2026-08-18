# Kuhhandel learning

PyTorch updates a linear policy from game outcomes. Go runs each game through the authoritative rules engine and returns batched rewards and policy gradients. The Python process never copies game rules or handles each move.

Use Python 3.10 or newer:

```sh
uv sync --project learning --python 3.11
uv run --project learning kuhhandel-learn --players 3 --steps 100 --batch-seeds 32
```

The trainer starts from the matching heuristic guide. It selects checkpoints on one fixed seed range, then reports one result on a separate held-out range. Checkpoints go to `learning/checkpoints` by default.

Train a reply to a saved policy while starting from that policy:

```sh
uv run --project learning kuhhandel-learn --players 3 --initial learning/models/three-player.json --opponent learning/models/three-player.json --exclude-guide
```

Repeat `--opponent` to train against several saved policies. The worker balances them across opponent seats and shuffle seeds. The heuristic guide stays in the pool unless `--exclude-guide` is set. `--initial` accepts an exported JSON model or a PyTorch checkpoint.

Evaluate the saved research models:

```sh
uv run --project learning kuhhandel-learn --players 3 --evaluate learning/models/three-player.json --held-out-seeds 1000 --seed 6500000
uv run --project learning kuhhandel-learn --players 5 --evaluate learning/models/five-player.json --held-out-seeds 1000 --seed 6900000
```

The five decision rows cover turns, bids, first refusal, trade responses, and second offers. Each row scores 16 facts derived from the visible state and a compact set of legal moves. Training explores 2% of decisions, applies Adam to the batched policy gradient, and keeps the best validation checkpoint. A learned move replaces its heuristic guide only when it clears a fixed score margin.
