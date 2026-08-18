package strategy

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"

	"github.com/khoi/kuhhandel/internal/game"
)

const (
	LinearDecisionCount = 5
	LinearFeatureCount  = 16
	linearExploration   = 0.02
	linearEvalMargin    = 0.5
)

const (
	linearTurnDecision = iota
	linearBidDecision
	linearResolveDecision
	linearRespondDecision
	linearReofferDecision
)

type LinearModel struct {
	Weights [LinearDecisionCount][LinearFeatureCount]float64
}

type LinearGradient [LinearDecisionCount][LinearFeatureCount]float64

type LinearPolicy struct {
	model              LinearModel
	guide              Policy
	random             *rand.Rand
	sample             bool
	steps              []linearStep
	deviations         [LinearDecisionCount]int
	lastTradeRemaining int
}

type linearFeatures [LinearFeatureCount]float64

type linearStep struct {
	decision int
	gradient linearFeatures
}

type linearCandidate struct {
	action   any
	features linearFeatures
	guided   bool
}

type linearBid struct {
	bid  game.PlaceBid
	will bool
}

func NewLinear(model LinearModel, guide HeuristicConfig, seed uint64, sample bool) *LinearPolicy {
	return &LinearPolicy{
		model:              model,
		guide:              NewHeuristic(guide, seed),
		random:             rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		sample:             sample,
		lastTradeRemaining: -1,
	}
}

func NewLinearModel(weights [][]float64) (LinearModel, error) {
	model := LinearModel{}
	if len(weights) == 0 {
		return model, nil
	}
	if len(weights) != LinearDecisionCount {
		return LinearModel{}, fmt.Errorf("model has %d decisions, want %d", len(weights), LinearDecisionCount)
	}
	for decision, row := range weights {
		if len(row) != LinearFeatureCount {
			return LinearModel{}, fmt.Errorf("model decision %d has %d features, want %d", decision, len(row), LinearFeatureCount)
		}
		for feature, weight := range row {
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				return LinearModel{}, fmt.Errorf("model weight %d,%d is not finite", decision, feature)
			}
			model.Weights[decision][feature] = weight
		}
	}
	return model, nil
}

func (policy *LinearPolicy) Turn(snapshot game.Snapshot) game.Command {
	guided := policy.guide.Turn(snapshot)
	candidates := markGuided(linearTurnCandidates(snapshot, policy.lastTradeRemaining), guided, linearTurnFeaturesFor(snapshot, guided))
	command := policy.choose(linearTurnDecision, candidates).(game.Command)
	if _, trading := command.(game.BeginTrade); trading {
		policy.lastTradeRemaining = snapshot.Public.DeckRemaining
	}
	return command
}

func (policy *LinearPolicy) Bid(snapshot game.Snapshot) (game.PlaceBid, bool) {
	bid, will := policy.guide.Bid(snapshot)
	guided := linearBid{bid: bid, will: will}
	features := linearFeatures{}
	if will {
		features = linearBidFeatures(snapshot, bid.Amount)
	}
	selected := policy.choose(linearBidDecision, markGuided(linearBidCandidates(snapshot), guided, features)).(linearBid)
	return selected.bid, selected.will
}

func (policy *LinearPolicy) ResolveAuction(snapshot game.Snapshot) game.ResolveAuction {
	guided := policy.guide.ResolveAuction(snapshot)
	features := linearFeatures{}
	if guided.Buy {
		features = linearBidFeatures(snapshot, moneyTotal(guided.Payment))
	}
	candidates := markGuided(linearResolveCandidates(snapshot), guided, features)
	return policy.choose(linearResolveDecision, candidates).(game.ResolveAuction)
}

func (policy *LinearPolicy) RespondTrade(snapshot game.Snapshot) game.Command {
	guided := policy.guide.RespondTrade(snapshot)
	features := linearFeatures{}
	if counter, ok := guided.(game.CounterTrade); ok {
		features = linearTradeFeatures(snapshot, counter.Offer)
	}
	candidates := markGuided(linearRespondCandidates(snapshot), guided, features)
	return policy.choose(linearRespondDecision, candidates).(game.Command)
}

func (policy *LinearPolicy) ReofferTrade(snapshot game.Snapshot) game.ReofferTrade {
	guided := policy.guide.ReofferTrade(snapshot)
	candidates := markGuided(linearReofferCandidates(snapshot), guided, linearTradeFeatures(snapshot, guided.Offer))
	return policy.choose(linearReofferDecision, candidates).(game.ReofferTrade)
}

func (policy *LinearPolicy) Gradient() LinearGradient {
	gradient := LinearGradient{}
	for _, step := range policy.steps {
		for feature, value := range step.gradient {
			gradient[step.decision][feature] += value
		}
	}
	return gradient
}

func (policy *LinearPolicy) Decisions() int {
	return len(policy.steps)
}

func (policy *LinearPolicy) Deviations() [LinearDecisionCount]int {
	return policy.deviations
}

func (policy *LinearPolicy) choose(decision int, candidates []linearCandidate) any {
	if len(candidates) == 1 {
		return candidates[0].action
	}
	probabilities, total := policy.probabilities(decision, candidates)
	guided := 0
	for index, candidate := range candidates {
		if candidate.guided {
			guided = index
			break
		}
	}
	selected := guided
	if policy.sample {
		policyProbabilities := make([]float64, len(candidates))
		for index, probability := range probabilities {
			policyProbabilities[index] = linearExploration * probability / total
		}
		policyProbabilities[guided] += 1 - linearExploration
		draw := policy.random.Float64()
		selected = len(candidates) - 1
		for index, probability := range policyProbabilities {
			draw -= probability
			if draw <= 0 {
				selected = index
				break
			}
		}
		gradient := candidates[selected].features
		for index, candidate := range candidates {
			probability := probabilities[index] / total
			for feature, value := range candidate.features {
				gradient[feature] -= probability * value
			}
		}
		scale := linearExploration * probabilities[selected] / total / policyProbabilities[selected]
		for feature := range gradient {
			gradient[feature] *= scale
		}
		policy.steps = append(policy.steps, linearStep{decision: decision, gradient: gradient})
	} else {
		best := guided
		for index := range probabilities {
			if probabilities[index] > probabilities[best] {
				best = index
			}
		}
		if probabilities[best] > probabilities[guided]*math.Exp(linearEvalMargin) {
			selected = best
		}
	}
	if selected != guided {
		policy.deviations[decision]++
	}
	return candidates[selected].action
}

func (policy *LinearPolicy) probabilities(decision int, candidates []linearCandidate) ([]float64, float64) {
	scores := make([]float64, len(candidates))
	maximum := math.Inf(-1)
	for index, candidate := range candidates {
		for feature, value := range candidate.features {
			scores[index] += policy.model.Weights[decision][feature] * value
		}
		maximum = max(maximum, scores[index])
	}
	total := 0.0
	for index, score := range scores {
		scores[index] = math.Exp(score - maximum)
		total += scores[index]
	}
	return scores, total
}

func markGuided(candidates []linearCandidate, action any, features linearFeatures) []linearCandidate {
	for index := range candidates {
		if reflect.DeepEqual(candidates[index].action, action) {
			candidates[index].guided = true
			candidates[index].features = features
			return candidates
		}
	}
	return append(candidates, linearCandidate{action: action, features: features, guided: true})
}
