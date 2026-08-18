package strategy

import (
	"fmt"
	"math"
	"runtime"
	"sync"
)

type RolloutOptions struct {
	Model          LearnedModel
	OpponentModels []LearnedModel
	Players        int
	Seeds          int
	Seed           uint64
	Sample         bool
	Exploration    float64
}

type RolloutResult struct {
	Games          int
	MeanReward     float64
	StandardError  float64
	MeanDecisions  float64
	MeanDeviations [LearnedDecisionCount]float64
	RewardGradient LearnedGradient
	MeanGradient   LearnedGradient
}

type rolloutSeedResult struct {
	reward         float64
	decisions      int
	deviations     [LearnedDecisionCount]int
	rewardGradient *LearnedGradient
	gradient       *LearnedGradient
	err            error
}

func Rollout(options RolloutOptions) (RolloutResult, error) {
	if options.Players < 3 || options.Players > 5 {
		return RolloutResult{}, fmt.Errorf("player count must be between three and five")
	}
	if options.Seeds <= 0 {
		return RolloutResult{}, fmt.Errorf("seed count must be positive")
	}
	if options.Exploration < 0 || options.Exploration > 1 {
		return RolloutResult{}, fmt.Errorf("exploration must be between zero and one")
	}
	results := make([]rolloutSeedResult, options.Seeds)
	jobs := make(chan int)
	workers := min(options.Seeds, runtime.GOMAXPROCS(0))
	var group sync.WaitGroup
	group.Add(workers)
	for range workers {
		go func() {
			defer group.Done()
			for index := range jobs {
				results[index] = rolloutSeed(&options, options.Seed+uint64(index))
			}
		}()
	}
	for index := range results {
		jobs <- index
	}
	close(jobs)
	group.Wait()
	return combineRollouts(results, options.Players)
}

func rolloutSeed(options *RolloutOptions, deckSeed uint64) rolloutSeedResult {
	result := rolloutSeedResult{}
	if options.Sample {
		result.rewardGradient = &LearnedGradient{}
		result.gradient = &LearnedGradient{}
	}
	opponents := options.OpponentModels
	if len(opponents) == 0 {
		opponents = []LearnedModel{{}}
	}
	guide := LargeGameChampion()
	if options.Players == 3 {
		guide = ThreePlayerChampion()
	}
	for challengerSeat := range options.Players {
		policies := make([]Policy, options.Players)
		var challenger *LearnedPolicy
		for seat := range options.Players {
			policySeed := deckSeed*uint64(options.Players+1) + uint64(seat+1)
			if seat == challengerSeat {
				challenger = NewLearned(&options.Model, guide, policySeed, options.Sample, options.Exploration)
				policies[seat] = challenger
			} else {
				index := (int(deckSeed%uint64(len(opponents))) + challengerSeat + seat) % len(opponents)
				policies[seat] = NewLearned(&opponents[index], guide, policySeed, false, 0)
			}
		}
		played, err := Play(deckSeed, policies)
		if err != nil {
			result.err = fmt.Errorf("seed %d seat %d: %w", deckSeed, challengerSeat, err)
			return result
		}
		reward := resultWinShare(played, played.Players[challengerSeat].ID)
		result.reward += reward
		result.decisions += challenger.Decisions()
		deviations := challenger.Deviations()
		for decision, count := range deviations {
			result.deviations[decision] += count
		}
		if options.Sample {
			gradient := challenger.Gradient()
			for decision := range LearnedDecisionCount {
				for parameter := range LearnedParameterCount {
					result.gradient[decision][parameter] += gradient[decision][parameter]
					result.rewardGradient[decision][parameter] += reward * gradient[decision][parameter]
				}
			}
		}
	}
	return result
}

func combineRollouts(seeds []rolloutSeedResult, players int) (RolloutResult, error) {
	result := RolloutResult{Games: len(seeds) * players}
	rewardSquare := 0.0
	for _, seed := range seeds {
		if seed.err != nil {
			return RolloutResult{}, seed.err
		}
		seedMean := seed.reward / float64(players)
		result.MeanReward += seedMean
		rewardSquare += seedMean * seedMean
		result.MeanDecisions += float64(seed.decisions)
		for decision, count := range seed.deviations {
			result.MeanDeviations[decision] += float64(count)
		}
		if seed.gradient != nil {
			for decision := range LearnedDecisionCount {
				for parameter := range LearnedParameterCount {
					result.MeanGradient[decision][parameter] += seed.gradient[decision][parameter]
					result.RewardGradient[decision][parameter] += seed.rewardGradient[decision][parameter]
				}
			}
		}
	}
	seedCount := float64(len(seeds))
	gameCount := float64(result.Games)
	result.MeanReward /= seedCount
	result.MeanDecisions /= gameCount
	for decision := range result.MeanDeviations {
		result.MeanDeviations[decision] /= gameCount
	}
	for decision := range LearnedDecisionCount {
		for parameter := range LearnedParameterCount {
			result.MeanGradient[decision][parameter] /= gameCount
			result.RewardGradient[decision][parameter] /= gameCount
		}
	}
	if len(seeds) > 1 {
		variance := (rewardSquare - seedCount*result.MeanReward*result.MeanReward) / float64(len(seeds)-1)
		result.StandardError = math.Sqrt(math.Max(0, variance) / seedCount)
	}
	return result, nil
}

func resultWinShare(result Result, playerID string) float64 {
	for _, winnerID := range result.WinnerIDs {
		if winnerID == playerID {
			return 1 / float64(len(result.WinnerIDs))
		}
	}
	return 0
}
