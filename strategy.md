# Kuhhandel strategy research

## Result

The best policies found use the same core plan:

1. Auction while animals remain. Make an early Kuhhandel only when winning it completes your set.
2. Keep about 30% of current money out of auction bids.
3. Value a card from its effect on the final multiplied score, not its printed set value alone.
4. Include denial value, but give it more weight with four or five players.
5. Use first refusal less than ordinary bidding because declining gives you the bidder's money.
6. Do not use routine zero-card offers. Keep offer size uncertain instead.
7. After the deck ends, prefer a trade that completes a set, then one between equal holdings.

Policy-gradient training found stronger replies for three and five players. Use the saved learned policy for those player counts and the heuristic champion for four players. These are empirical results against the policies tested. They are not a solved equilibrium.

## Valuation

For a player with current score `S`, complete the animal in a copied holding and compute the new score `S'`. If the player lacks `r` cards and the decision transfers `c` cards, the claim value is:

```text
(S' - S) × c / r
```

The policy adds its own claim value to a fraction of the strongest affected opponent's claim value. That fraction is denial value.

If the player has `k` complete sets worth `V` before the score multiplier, completing a set printed as `v` adds exactly:

```text
V + (k + 1) × v
```

## Heuristic guides

| Players | Auction fraction | Denial fraction | Cash reserve | First-refusal fraction | Kuhhandel fraction | Bluff chance |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 3 | 60% | 35% | 30% | 25% | 75% | 0% |
| 4–5 | 60% | 65% | 30% | 35% | 100% | 0% |

The auction ceiling is the auction fraction times contest value, capped so the cash reserve remains. The bot varies each bid between the current bid and that ceiling. A Kuhhandel offer varies between half and all of its ceiling. Zero remains the fallback when no positive offer fits.

Under the server rules, a target holding a zero card should counter with zero instead of accepting. Against a positive offer, both choices lose the animal and receive the positive cards. Against a zero offer, the counter earns another round without a worse result. Every player starts with zero cards, and revealed zeros return to their owner.

## Evidence

The experiments completed more than 380,000 games. Each comparison rotates the challenger through every seat. Every challenger policy faces the same shuffle seeds and policy random streams for a given opponent. A table cell measures one challenger against copies of the column policy.

The broad policy comparison found auction-first play best:

| Players | Games per seat and pairing | Best policy win share | Standard error | Equal-policy share |
| ---: | ---: | ---: | ---: | ---: |
| 3 | 100 | 47.3% | 1.0% | 33.3% |
| 4 | 50 | 35.4% | 1.2% | 25.0% |
| 5 | 30 | 29.0% | 1.4% | 20.0% |

Against the stronger finalist pool:

| Players | Recommended policy win share | Standard error | Equal-policy share |
| ---: | ---: | ---: | ---: |
| 3 | 43.1% | 0.8% | 33.3% |
| 4 | 28.8% | 0.9% | 25.0% |
| 5 | 23.0% | 0.7% | 20.0% |

One-parameter replies around the champions did not produce a confirmed exploit. The strongest apparent three-player reply scored 36.3% over 300 games, then 32.0% over 3,000 new games against a 33.3% equal share.

A deterministic low-discrepancy search then tested 128 joint parameter combinations. The best screening results did not survive new shuffle seeds:

| Players | Best screening share | Best held-out share | Equal-policy share |
| ---: | ---: | ---: | ---: |
| 3 | 41.1% | 34.0% | 33.3% |
| 4 | 25.8% | 24.0% | 25.0% |
| 5 | 20.7% | 18.1% | 20.0% |

PyTorch policy-gradient training then searched beyond the fixed heuristic parameters. Go ran every game and returned batched rewards and action gradients. The frozen policies were tested on 1,000 new shuffle seeds with every seat rotation:

| Players | Learned share | Standard error | Equal-policy share | Result |
| ---: | ---: | ---: | ---: | --- |
| 3 | 40.9% | 0.8% | 33.3% | confirmed reply |
| 4 | 24.9% | 0.4% | 25.0% | no gain |
| 5 | 22.4% | 0.5% | 20.0% | confirmed reply |

The three-player policy changes a mean 1.10 turn choices and 0.29 bids per game from its guide. The five-player policy changes 0.52 turn choices, 0.22 bids, and 0.38 first-refusal choices. Neither learned policy changes trade responses or second offers under the fixed evaluation margin.

## Model

The simulator calls `internal/game` for every rule check and state change. It does not copy the game rules.

The server permits bids and auction closure without a bidder turn. The research runner makes this discrete:

1. Start with the seat after the auctioneer.
2. Each non-leading bidder may bid or pass.
3. A bid reopens responses.
4. Close after every bidder other than the leader passes.

The policy sees only its snapshot: public state, its money, and its legal actions. It never sees other money, hidden offers, or deck order.

The learned policy scores a compact set of legal actions from 16 visible features. It starts from the matching heuristic guide, explores 2% of decisions during training, and replaces a guide move only when its learned score clears a fixed margin. Python updates the 80 weights with Adam. The Go worker keeps all rule use and full-game simulation on the Go side of one persistent JSON stream.

## Reproduce

```sh
go run ./cmd/kuhhandel-strategy -suite archetypes -players 3 -games 100 -seed 50000
go run ./cmd/kuhhandel-strategy -suite tuning -players 4 -games 10 -seed 320000
go run ./cmd/kuhhandel-strategy -suite champions -opponents finalists -players 5 -games 50 -seed 1200000
go run ./cmd/kuhhandel-strategy -suite probes -policy small-denial-10 -opponents champions -opponent-policy three-champion -players 3 -games 1000 -seed 1900000
go run ./cmd/kuhhandel-strategy -suite search -samples 128 -opponents champions -opponent-policy three-champion -players 3 -games 30 -seed 2100000
go run ./cmd/kuhhandel-strategy -suite search -samples 128 -policy search-028 -opponents champions -opponent-policy three-champion -players 3 -games 1000 -seed 3000000
uv sync --project learning --python 3.11
uv run --project learning kuhhandel-learn --players 3 --steps 100 --batch-seeds 16 --eval-every 20 --eval-seeds 100 --held-out-seeds 300 --seed 6400000 --export learning/models/three-player.json
uv run --project learning kuhhandel-learn --players 5 --steps 100 --batch-seeds 16 --eval-every 20 --eval-seeds 100 --held-out-seeds 300 --seed 6700000 --export learning/models/five-player.json
uv run --project learning kuhhandel-learn --players 3 --evaluate learning/models/three-player.json --held-out-seeds 1000 --seed 6500000
uv run --project learning kuhhandel-learn --players 5 --evaluate learning/models/five-player.json --held-out-seeds 1000 --seed 6900000
```

## Limits

- The result applies to the implemented second-edition rules and the discrete auction schedule above.
- The opponent pool contains parameterized heuristic policies, not all possible policies.
- The learned replies optimize against copies of one heuristic champion, not a mixed or adapting field.
- The learned action scorer is linear and begins from the heuristic guide. It does not test every policy form.
- The policy tracks only whether it already made an optional trade at the current deck count. It does not infer hidden money from history.
- No current result proves a Nash equilibrium or a globally best strategy.
- The next strong test is iterative self-play against a mixture of the heuristic and learned policies.
