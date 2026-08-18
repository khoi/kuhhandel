package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/khoi/kuhhandel/internal/strategy"
)

type request struct {
	Kind    string      `json:"kind"`
	Players int         `json:"players"`
	Seeds   int         `json:"seeds"`
	Seed    uint64      `json:"seed"`
	Sample  bool        `json:"sample"`
	Weights [][]float64 `json:"weights"`
}

type response struct {
	Error          string      `json:"error,omitempty"`
	Decisions      int         `json:"decisions"`
	Features       int         `json:"features"`
	Games          int         `json:"games,omitempty"`
	MeanReward     float64     `json:"mean_reward"`
	StandardError  float64     `json:"standard_error"`
	MeanDecisions  float64     `json:"mean_decisions"`
	MeanDeviations []float64   `json:"mean_deviations,omitempty"`
	RewardGradient [][]float64 `json:"reward_gradient,omitempty"`
	MeanGradient   [][]float64 `json:"mean_gradient,omitempty"`
}

func main() {
	if err := serve(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func serve(input io.Reader, output io.Writer) error {
	decoder := json.NewDecoder(input)
	decoder.DisallowUnknownFields()
	encoder := json.NewEncoder(output)
	for {
		var request request
		if err := decoder.Decode(&request); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		result := handle(request)
		if err := encoder.Encode(result); err != nil {
			return err
		}
	}
}

func handle(request request) response {
	result := response{Decisions: strategy.LinearDecisionCount, Features: strategy.LinearFeatureCount}
	if request.Kind == "shape" {
		return result
	}
	if request.Kind != "rollout" {
		result.Error = fmt.Sprintf("unknown request kind %q", request.Kind)
		return result
	}
	model, err := strategy.NewLinearModel(request.Weights)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	rolled, err := strategy.Rollout(strategy.RolloutOptions{
		Model:   model,
		Players: request.Players,
		Seeds:   request.Seeds,
		Seed:    request.Seed,
		Sample:  request.Sample,
	})
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Games = rolled.Games
	result.MeanReward = rolled.MeanReward
	result.StandardError = rolled.StandardError
	result.MeanDecisions = rolled.MeanDecisions
	result.MeanDeviations = append([]float64(nil), rolled.MeanDeviations[:]...)
	if request.Sample {
		result.RewardGradient = gradientRows(rolled.RewardGradient)
		result.MeanGradient = gradientRows(rolled.MeanGradient)
	}
	return result
}

func gradientRows(gradient strategy.LinearGradient) [][]float64 {
	rows := make([][]float64, strategy.LinearDecisionCount)
	for decision := range rows {
		rows[decision] = append([]float64(nil), gradient[decision][:]...)
	}
	return rows
}
