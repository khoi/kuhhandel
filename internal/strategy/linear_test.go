package strategy

import (
	"math"
	"reflect"
	"testing"

	"github.com/khoi/kuhhandel/internal/game"
)

func TestLearnedPolicyMatchesGuideWithZeroWeights(t *testing.T) {
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

func TestSampledLearnedPolicyReturnsGradients(t *testing.T) {
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
	if first.MeanDecisions == 0 || first.MeanGradient == (LearnedGradient{}) {
		t.Fatal("sampled rollout has no policy gradient")
	}
}

func TestSampledLearnedPolicyExploresAroundCurrentPolicy(t *testing.T) {
	model := LearnedModel{}
	model.Weights[linearTurnDecision][0] = 1
	policy := NewLearned(&model, ThreePlayerChampion(), 902, true, 0)
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

func TestExpandedChoicesRequireLearnedResidual(t *testing.T) {
	model := LearnedModel{}
	model.Weights[linearTurnDecision][0] = 1
	candidates := []linearCandidate{
		{action: 0, guided: true},
		{action: 1, features: linearFeatures{0.6}},
		{action: 2, features: linearFeatures{2}, expanded: true},
	}
	legacy := NewLearned(&model, ThreePlayerChampion(), 1, false, 0)
	if selected := legacy.choose(linearTurnDecision, candidates).(int); selected != 1 {
		t.Fatalf("legacy choice = %d, want 1", selected)
	}
	model.Weights[linearBidDecision][learnedOutputOffset(0)] = 0.1
	otherDecision := NewLearned(&model, ThreePlayerChampion(), 1, false, 0)
	if selected := otherDecision.choose(linearTurnDecision, candidates).(int); selected != 1 {
		t.Fatalf("choice with another residual = %d, want 1", selected)
	}
	model.Weights[linearTurnDecision][learnedOutputOffset(0)] = 0.1
	expanded := NewLearned(&model, ThreePlayerChampion(), 1, false, 0)
	if selected := expanded.choose(linearTurnDecision, candidates).(int); selected != 2 {
		t.Fatalf("expanded choice = %d, want 2", selected)
	}
}

func TestRolloutWithMixedOpponentsIsDeterministic(t *testing.T) {
	opponent := LearnedModel{}
	opponent.Weights[linearTurnDecision][0] = 1
	options := RolloutOptions{
		OpponentModels: []LearnedModel{{}, opponent},
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

func TestNewLearnedModelValidatesShapeAndValues(t *testing.T) {
	valid := make([][]float64, LearnedDecisionCount)
	for decision := range valid {
		valid[decision] = make([]float64, LearnedParameterCount)
	}
	if _, err := NewLearnedModel(valid); err != nil {
		t.Fatal(err)
	}
	for _, weights := range [][][]float64{
		{{1}},
		append(valid[:LearnedDecisionCount-1:LearnedDecisionCount-1], []float64{1}),
	} {
		if _, err := NewLearnedModel(weights); err == nil {
			t.Fatal("invalid model succeeded")
		}
	}
	invalid := make([][]float64, LearnedDecisionCount)
	for decision := range invalid {
		invalid[decision] = make([]float64, LearnedParameterCount)
	}
	invalid[0][0] = math.NaN()
	if _, err := NewLearnedModel(invalid); err == nil {
		t.Fatal("non-finite model succeeded")
	}
}

func TestLearnedScoreGradientMatchesFiniteDifference(t *testing.T) {
	model := LearnedModel{}
	for parameter := range LearnedParameterCount {
		model.Weights[linearBidDecision][parameter] = float64(parameter%11-5) / 50
	}
	features := linearFeatures{}
	for feature := range features {
		features[feature] = float64(feature%7-3) / 4
	}
	policy := NewLearned(&model, ThreePlayerChampion(), 1, false, 0)
	gradient := policy.scoreGradient(linearBidDecision, features)
	const epsilon = 1e-6
	for parameter := range LearnedParameterCount {
		plus := model
		minus := model
		plus.Weights[linearBidDecision][parameter] += epsilon
		minus.Weights[linearBidDecision][parameter] -= epsilon
		plusScore := NewLearned(&plus, ThreePlayerChampion(), 1, false, 0).score(linearBidDecision, features)
		minusScore := NewLearned(&minus, ThreePlayerChampion(), 1, false, 0).score(linearBidDecision, features)
		finite := (plusScore - minusScore) / (2 * epsilon)
		if math.Abs(gradient[parameter]-finite) > 1e-8 {
			t.Fatalf("gradient %d = %f, want %f", parameter, gradient[parameter], finite)
		}
	}
}

func TestZeroNeuralOutputPreservesLinearScore(t *testing.T) {
	model := LearnedModel{}
	features := linearFeatures{1, 0.25, -0.5}
	model.Weights[linearTurnDecision][0] = 2
	model.Weights[linearTurnDecision][1] = 4
	for parameter := LearnedFeatureCount; parameter < learnedBiasOffset(0); parameter++ {
		model.Weights[linearTurnDecision][parameter] = 1
	}
	policy := NewLearned(&model, ThreePlayerChampion(), 1, false, 0)
	if score := policy.score(linearTurnDecision, features); score != 3 {
		t.Fatalf("score = %f, want 3", score)
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

func TestLearnedPolicyUsesWiderBidAndOfferChoices(t *testing.T) {
	money := game.Money{Ten: 10, Fifty: 2, Hundred: 2, TwoHundred: 1, FiveHundred: 1}
	if bids := linearBidOptions(money, 1); len(bids) < 10 {
		t.Fatalf("bid choices = %d, want at least 10", len(bids))
	}
	if offers := linearOffers(money); len(offers) < 11 {
		t.Fatalf("offer choices = %d, want at least 11", len(offers))
	}
}
