package strategy

import (
	"math"
	"reflect"
	"testing"
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
