package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/khoi/kuhhandel/internal/strategy"
)

func TestServeReportsShapeAndRollout(t *testing.T) {
	input := strings.NewReader("{\"kind\":\"shape\"}\n{\"kind\":\"rollout\",\"players\":3,\"seeds\":2,\"seed\":81}\n")
	var output bytes.Buffer
	if err := serve(input, &output); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(&output)
	var shape response
	if err := decoder.Decode(&shape); err != nil {
		t.Fatal(err)
	}
	if shape.Decisions != strategy.LearnedDecisionCount || shape.Features != strategy.LearnedFeatureCount ||
		shape.Hidden != strategy.LearnedHiddenCount || shape.Parameters != strategy.LearnedParameterCount {
		t.Fatalf("shape = %+v", shape)
	}
	var rollout response
	if err := decoder.Decode(&rollout); err != nil {
		t.Fatal(err)
	}
	if rollout.Games != 6 || rollout.MeanReward != 1.0/3.0 {
		t.Fatalf("rollout = %+v", rollout)
	}
}

func TestHandleRejectsInvalidRequests(t *testing.T) {
	for _, request := range []request{
		{},
		{Kind: "rollout", Players: 2, Seeds: 1},
		{Kind: "rollout", Players: 3, Seeds: 0},
		{Kind: "rollout", Players: 3, Seeds: 1, Exploration: 2},
		{Kind: "rollout", Players: 3, Seeds: 1, Weights: [][]float64{{1}}},
		{Kind: "rollout", Players: 3, Seeds: 1, Opponents: [][][]float64{{{1}}}},
	} {
		if result := handle(request); result.Error == "" {
			t.Fatalf("request succeeded: %+v", request)
		}
	}
}

func TestSampledRolloutReportsGradientShape(t *testing.T) {
	result := handle(request{Kind: "rollout", Players: 3, Seeds: 1, Seed: 82, Sample: true})
	if result.Error != "" {
		t.Fatal(result.Error)
	}
	if len(result.RewardGradient) != strategy.LearnedDecisionCount {
		t.Fatalf("gradient decisions = %d", len(result.RewardGradient))
	}
	for _, row := range result.RewardGradient {
		if len(row) != strategy.LearnedParameterCount {
			t.Fatalf("gradient parameters = %d", len(row))
		}
	}
}
