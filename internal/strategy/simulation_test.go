package strategy

import (
	"reflect"
	"testing"

	"github.com/khoi/kuhhandel/internal/game"
)

func TestPlayFinishesDeterministically(t *testing.T) {
	config := HeuristicConfig{
		Name:                 "balanced",
		AuctionFraction:      0.5,
		DenialFraction:       0.35,
		ReserveFraction:      0.4,
		FirstRefusalFraction: 0.5,
		TradeFraction:        0.7,
		TradeAtDeckRemaining: 8,
		BluffChance:          0.4,
	}
	for playerCount := 3; playerCount <= 5; playerCount++ {
		first, err := Play(91, policies(config, playerCount, 100))
		if err != nil {
			t.Fatal(err)
		}
		second, err := Play(91, policies(config, playerCount, 100))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(first, second) {
			t.Fatalf("%d-player simulation is not deterministic", playerCount)
		}
		if len(first.WinnerIDs) == 0 {
			t.Fatalf("%d-player simulation has no winner", playerCount)
		}
		totals := map[game.Animal]int{}
		for _, player := range first.Players {
			for animal, count := range player.Animals {
				totals[animal] += count
				if count != 0 && count != 4 {
					t.Fatalf("%s has %d %s cards", player.ID, count, animal)
				}
			}
		}
		if len(totals) != 10 {
			t.Fatalf("animal types = %d, want 10", len(totals))
		}
		for animal, count := range totals {
			if count != 4 {
				t.Fatalf("%s cards = %d, want 4", animal, count)
			}
		}
	}
}

func TestPaymentSelectionUsesTheSmallestSufficientTotal(t *testing.T) {
	money := game.Money{Ten: 4, Fifty: 1, Hundred: 1}
	payment, total, ok := paymentAtLeast(money, 60, 90)
	if !ok {
		t.Fatal("no payment")
	}
	if total != 60 || payment != (game.Money{Ten: 1, Fifty: 1}) {
		t.Fatalf("payment = %+v (%d), want 10+50", payment, total)
	}
	maximum, ok := paymentAtMost(money, 90)
	if !ok || maximum != (game.Money{Ten: 4, Fifty: 1}) {
		t.Fatalf("maximum payment = %+v, want 40+50", maximum)
	}
}

func TestTradeHeavyFourPlayerGameFinishes(t *testing.T) {
	spender := HeuristicConfig{
		Name: "spender", AuctionFraction: 0.8, DenialFraction: 0.4, ReserveFraction: 0.15,
		FirstRefusalFraction: 0.6, TradeFraction: 1, TradeAtDeckRemaining: 40, BluffChance: 0.15,
	}
	cautious := HeuristicConfig{
		Name: "cautious", AuctionFraction: 0.3, DenialFraction: 0.15, ReserveFraction: 0.65,
		FirstRefusalFraction: 0.4, TradeFraction: 0.45, TradeAtDeckRemaining: 0, BluffChance: 0.65,
	}
	configs := []HeuristicConfig{spender, cautious, spender, spender}
	players := make([]Policy, len(configs))
	for seat, config := range configs {
		players[seat] = NewHeuristic(config, 90301+uint64(seat))
	}
	result, err := Play(90301, players)
	if err != nil {
		t.Fatal(err)
	}
	if result.Trades > 100 {
		t.Fatalf("trades = %d, want at most 100", result.Trades)
	}
}

func TestTargetCountersWithZeroInsteadOfAccepting(t *testing.T) {
	policy := NewHeuristic(HeuristicConfig{}, 1)
	snapshot := game.Snapshot{
		Public: game.PublicView{
			Phase: game.PhaseTradeResponse,
			Players: []game.PublicPlayer{
				{ID: "challenger", Animals: map[game.Animal]int{game.AnimalHorse: 3}},
				{ID: "target", Animals: map[game.Animal]int{game.AnimalHorse: 1}},
			},
			Trade: &game.TradeView{ChallengerID: "challenger", TargetID: "target", Animal: game.AnimalHorse, CardCount: 1},
		},
		Self: game.SelfView{PlayerID: "target", Money: game.Money{Zero: 1}},
	}
	command := policy.RespondTrade(snapshot)
	counter, ok := command.(game.CounterTrade)
	if !ok || counter.Offer != (game.Money{Zero: 1}) {
		t.Fatalf("response = %+v, want zero counter", command)
	}
}

func policies(config HeuristicConfig, count int, seed uint64) []Policy {
	result := make([]Policy, count)
	for seat := range result {
		result[seat] = NewHeuristic(config, seed+uint64(seat))
	}
	return result
}
