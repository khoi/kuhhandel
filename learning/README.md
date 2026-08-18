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

Use `--freeze-features 16` to keep the original action weights fixed while training the added interaction and public-history facts.

Evaluate the saved research models:

```sh
uv run --project learning kuhhandel-learn --players 3 --evaluate learning/models/three-player.json --held-out-seeds 1000 --seed 6500000
uv run --project learning kuhhandel-learn --players 5 --evaluate learning/models/five-player.json --held-out-seeds 1000 --seed 6900000
```

The five decision rows cover turns, bids, first refusal, trade responses, and second offers. Each row scores 16 current-state facts, eight interactions, and eight facts derived from public bid and trade history. Training explores 2% of decisions around the current policy, applies Adam to the batched policy gradient, and restores the best validation checkpoint after a failed block. A learned move replaces its heuristic guide only when it clears a fixed score margin.

The optimizer stays on CPU. The model has only 160 weights, while Go game simulation sets total run time. On the development Mac, 2,000 representative Adam updates took 0.160 seconds on CPU and 2.901 seconds on the [PyTorch MPS backend](https://docs.pytorch.org/docs/stable/notes/mps.html). MPS dispatch and transfer cost exceeds its work here. The Apple Neural Engine runs converted [Core ML predictions](https://apple.github.io/coremltools/docs-guides/source/model-prediction.html); it does not run this PyTorch training loop or the Go simulator.
