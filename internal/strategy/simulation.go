package strategy

import (
	"fmt"

	"github.com/khoi/kuhhandel/internal/game"
)

type Result struct {
	Players   []game.PublicPlayer
	WinnerIDs []string
	Trades    int
}

func Play(seed uint64, policies []Policy) (Result, error) {
	if len(policies) < 3 || len(policies) > 5 {
		return Result{}, fmt.Errorf("player count must be between three and five")
	}
	aggregate := game.New(fmt.Sprintf("simulation-%d", seed))
	for seat := range policies {
		identity := simulationIdentity(seat)
		command := game.Command(game.JoinRoom{Player: identity})
		if seat == 0 {
			command = game.CreateRoom{Player: identity}
		}
		if err := apply(aggregate, "", command); err != nil {
			return Result{}, err
		}
	}
	if err := apply(aggregate, simulationIdentity(0).ID, game.StartGame{Seed: seed}); err != nil {
		return Result{}, err
	}
	auctions := 0
	bids := 0
	trades := 0
	for aggregate.Public().Status == game.StatusPlaying {
		version, err := simulationVersion(aggregate)
		if err != nil {
			return Result{}, err
		}
		public := aggregate.Public()
		if version > 10_000 {
			return Result{}, fmt.Errorf("simulation exceeded 10000 events in phase %q with %d animals left after %d auctions, %d bids, and %d trades", public.Phase, public.DeckRemaining, auctions, bids, trades)
		}
		switch public.Phase {
		case game.PhaseTurn:
			seat := playerSeat(public, public.TurnPlayerID)
			snapshot, err := aggregate.Snapshot(public.TurnPlayerID)
			if err != nil {
				return Result{}, err
			}
			command := policies[seat].Turn(snapshot)
			if _, tradesNow := command.(game.BeginTrade); tradesNow {
				trades++
			}
			if err := apply(aggregate, public.TurnPlayerID, command); err != nil {
				return Result{}, err
			}
		case game.PhaseAuction:
			auctionBids, err := runAuction(aggregate, policies)
			if err != nil {
				return Result{}, err
			}
			auctions++
			bids += auctionBids
		case game.PhaseFirstRefusal:
			auctioneerID := public.Auction.AuctioneerID
			seat := playerSeat(public, auctioneerID)
			snapshot, err := aggregate.Snapshot(auctioneerID)
			if err != nil {
				return Result{}, err
			}
			if err := apply(aggregate, auctioneerID, policies[seat].ResolveAuction(snapshot)); err != nil {
				return Result{}, err
			}
		case game.PhaseTradeResponse, game.PhaseTradeRecounter:
			targetID := public.Trade.TargetID
			seat := playerSeat(public, targetID)
			snapshot, err := aggregate.Snapshot(targetID)
			if err != nil {
				return Result{}, err
			}
			if err := apply(aggregate, targetID, policies[seat].RespondTrade(snapshot)); err != nil {
				return Result{}, err
			}
		case game.PhaseTradeReoffer:
			challengerID := public.Trade.ChallengerID
			seat := playerSeat(public, challengerID)
			snapshot, err := aggregate.Snapshot(challengerID)
			if err != nil {
				return Result{}, err
			}
			if err := apply(aggregate, challengerID, policies[seat].ReofferTrade(snapshot)); err != nil {
				return Result{}, err
			}
		default:
			return Result{}, fmt.Errorf("unsupported phase %q", public.Phase)
		}
	}
	public := aggregate.Public()
	return Result{Players: public.Players, WinnerIDs: public.WinnerIDs, Trades: trades}, nil
}

func runAuction(aggregate *game.Aggregate, policies []Policy) (int, error) {
	public := aggregate.Public()
	auctioneerSeat := playerSeat(public, public.Auction.AuctioneerID)
	seat := (auctioneerSeat + 1) % len(policies)
	passes := 0
	bidCount := 0
	for {
		public = aggregate.Public()
		leaderSeat := playerSeat(public, public.Auction.HighestBidderID)
		neededPasses := len(policies) - 1
		if leaderSeat >= 0 {
			neededPasses--
		}
		if passes >= neededPasses {
			return bidCount, apply(aggregate, public.Auction.AuctioneerID, game.CloseAuction{})
		}
		if seat == auctioneerSeat || seat == leaderSeat {
			seat = (seat + 1) % len(policies)
			continue
		}
		playerID := public.Players[seat].ID
		snapshot, err := aggregate.Snapshot(playerID)
		if err != nil {
			return 0, err
		}
		bid, willBid := policies[seat].Bid(snapshot)
		if willBid {
			if err := apply(aggregate, playerID, bid); err != nil {
				return 0, err
			}
			bidCount++
			passes = 0
		} else {
			passes++
		}
		seat = (seat + 1) % len(policies)
	}
}

func apply(aggregate *game.Aggregate, actorID string, command game.Command) error {
	event, err := aggregate.Decide(actorID, command)
	if err != nil {
		return fmt.Errorf("%s %T: %w", actorID, command, err)
	}
	if err := aggregate.Apply(event); err != nil {
		return fmt.Errorf("apply %T: %w", command, err)
	}
	return nil
}

func simulationIdentity(seat int) game.Identity {
	playerID := fmt.Sprintf("player-%d", seat+1)
	return game.Identity{ID: playerID, AuthHash: game.HashToken(playerID), Name: playerID}
}

func simulationVersion(aggregate *game.Aggregate) (uint64, error) {
	snapshot, err := aggregate.Snapshot(simulationIdentity(0).ID)
	if err != nil {
		return 0, err
	}
	return snapshot.Version, nil
}

func playerSeat(public game.PublicView, playerID string) int {
	for _, player := range public.Players {
		if player.ID == playerID {
			return player.Seat
		}
	}
	return -1
}
