package strategy

import (
	"math"
	"reflect"
	"testing"

	"github.com/khoi/kuhhandel/internal/game"
)

func TestLinearPolicyMatchesGuideWithZeroWeights(t *testing.T) {
	for players := 3; players <= 5; players++ {
		result, err := Rollout(RolloutOptions{Players: players, Seeds: 10, Seed: 701})
		if err != nil {
			t.Fatal(err)
		}
		want := 1 / float64(players)
		if math.Abs(result.MeanReward-want) > 1e-12 {
			t.Fatalf("%d-player reward = %f, want %f", players, result.MeanReward, want)
		}
		if result.StandardError > 1e-9 {
			t.Fatalf("%d-player standard error = %f, want zero", players, result.StandardError)
		}
	}
}

func TestSampledLinearPolicyReturnsGradients(t *testing.T) {
	first, err := Rollout(RolloutOptions{Players: 3, Seeds: 5, Seed: 901, Sample: true})
	if err != nil {
		t.Fatal(err)
	}
	second, err := Rollout(RolloutOptions{Players: 3, Seeds: 5, Seed: 901, Sample: true})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("sampled rollout is not deterministic")
	}
	if first.MeanDecisions == 0 || first.MeanGradient == (LinearGradient{}) {
		t.Fatal("sampled rollout has no policy gradient")
	}
}

func TestSampledLinearPolicyExploresAroundCurrentPolicy(t *testing.T) {
	model := LinearModel{}
	model.Weights[linearTurnDecision][0] = 1
	policy := NewLinear(model, ThreePlayerChampion(), 902, true)
	candidates := []linearCandidate{
		{action: 0, guided: true},
		{action: 1, features: linearFeatures{1}},
	}
	deviations := 0
	for range 100 {
		if policy.choose(linearTurnDecision, candidates).(int) == 1 {
			deviations++
		}
	}
	if deviations < 90 {
		t.Fatalf("sampled %d current-policy actions", deviations)
	}
}

func TestRolloutWithMixedOpponentsIsDeterministic(t *testing.T) {
	opponent := LinearModel{}
	opponent.Weights[linearTurnDecision][0] = 1
	options := RolloutOptions{
		OpponentModels: []LinearModel{{}, opponent},
		Players:        3,
		Seeds:          5,
		Seed:           950,
	}
	first, err := Rollout(options)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Rollout(options)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("mixed rollout is not deterministic")
	}
}

func TestNewLinearModelValidatesShapeAndValues(t *testing.T) {
	valid := make([][]float64, LinearDecisionCount)
	for decision := range valid {
		valid[decision] = make([]float64, LinearFeatureCount)
	}
	if _, err := NewLinearModel(valid); err != nil {
		t.Fatal(err)
	}
	for _, weights := range [][][]float64{
		{{1}},
		append(valid[:LinearDecisionCount-1:LinearDecisionCount-1], []float64{1}),
	} {
		if _, err := NewLinearModel(weights); err == nil {
			t.Fatal("invalid model succeeded")
		}
	}
	invalid := make([][]float64, LinearDecisionCount)
	for decision := range invalid {
		invalid[decision] = make([]float64, LinearFeatureCount)
	}
	invalid[0][0] = math.NaN()
	if _, err := NewLinearModel(invalid); err == nil {
		t.Fatal("non-finite model succeeded")
	}
}

func TestLinearHistoryTracksPublicAuctionsAndTrades(t *testing.T) {
	history := linearHistory{}
	players := []game.PublicPlayer{
		{ID: "p1", Animals: map[game.Animal]int{game.AnimalGoat: 1}},
		{ID: "p2", Animals: map[game.Animal]int{game.AnimalGoat: 2}},
		{ID: "p3", Animals: map[game.Animal]int{}},
	}
	history.observe(game.PublicView{Phase: game.PhaseTurn, Players: players})
	history.observe(game.PublicView{
		Phase:   game.PhaseAuction,
		Players: players,
		Auction: &game.AuctionView{Animal: game.AnimalDonkey, AuctioneerID: "p1"},
	})
	history.observe(game.PublicView{
		Phase:   game.PhaseAuction,
		Players: players,
		Auction: &game.AuctionView{Animal: game.AnimalDonkey, AuctioneerID: "p1", HighestBidderID: "p2", HighestBid: 100},
	})
	settledAuction := []game.PublicPlayer{
		{ID: "p1", Animals: map[game.Animal]int{game.AnimalGoat: 1}},
		{ID: "p2", Animals: map[game.Animal]int{game.AnimalGoat: 2, game.AnimalDonkey: 1}},
		{ID: "p3", Animals: map[game.Animal]int{}},
	}
	history.observe(game.PublicView{Phase: game.PhaseTurn, Players: settledAuction})
	if got := history.players["p1"]; got.money != 240 || got.auctionWins != 0 {
		t.Fatalf("p1 auction history = %+v", got)
	}
	if got := history.players["p2"]; got.money != 40 || got.bids != 1 || got.maxBid != 100 || got.auctionWins != 1 {
		t.Fatalf("p2 auction history = %+v", got)
	}
	history.observe(game.PublicView{
		Phase:   game.PhaseTradeResponse,
		Players: settledAuction,
		Trade:   &game.TradeView{ChallengerID: "p1", TargetID: "p2", Animal: game.AnimalGoat, CardCount: 1},
	})
	settledTrade := []game.PublicPlayer{
		{ID: "p1", Animals: map[game.Animal]int{game.AnimalGoat: 2}},
		{ID: "p2", Animals: map[game.Animal]int{game.AnimalGoat: 1, game.AnimalDonkey: 1}},
		{ID: "p3", Animals: map[game.Animal]int{}},
	}
	history.observe(game.PublicView{Phase: game.PhaseTurn, Players: settledTrade})
	if got := history.players["p1"]; got.trades != 1 || got.tradeWins != 1 {
		t.Fatalf("p1 trade history = %+v", got)
	}
}

func TestLinearInteractionsExpandBaseFeatures(t *testing.T) {
	features := linearFeatures{}
	features[1] = 0.5
	features[6] = 0.4
	features[7] = 0.3
	interactions := linearInteractions(linearTurnDecision, features)
	if interactions[0] != 0.2 || interactions[1] != 0.15 {
		t.Fatalf("interactions = %v", interactions)
	}
}
