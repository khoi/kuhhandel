package strategy

import (
	"fmt"
	"math"
	"math/rand/v2"
	"reflect"

	"github.com/khoi/kuhhandel/internal/game"
)

const (
	LearnedDecisionCount          = 5
	LearnedFeatureCount           = 32
	LearnedHiddenCount            = 8
	LearnedParameterCount         = LearnedFeatureCount + LearnedHiddenCount*(LearnedFeatureCount+2)
	linearBaseFeatureCount        = 16
	linearInteractionFeatureCount = 8
	linearExploration             = 0.02
	linearEvalMargin              = 0.5
)

const (
	linearTurnDecision = iota
	linearBidDecision
	linearResolveDecision
	linearRespondDecision
	linearReofferDecision
)

type LearnedModel struct {
	Weights [LearnedDecisionCount][LearnedParameterCount]float64
}

type LearnedGradient [LearnedDecisionCount][LearnedParameterCount]float64

type LearnedPolicy struct {
	model              *LearnedModel
	guide              Policy
	random             *rand.Rand
	sample             bool
	exploration        float64
	gradient           LearnedGradient
	decisions          int
	deviations         [LearnedDecisionCount]int
	lastTradeRemaining int
	history            linearHistory
	expanded           [LearnedDecisionCount]bool
}

type linearFeatures [LearnedFeatureCount]float64

type learnedParameters [LearnedParameterCount]float64

type linearCandidate struct {
	action   any
	features linearFeatures
	guided   bool
	expanded bool
}

type linearBid struct {
	bid  game.PlaceBid
	will bool
}

func NewLearned(model *LearnedModel, guide HeuristicConfig, seed uint64, sample bool, exploration float64) *LearnedPolicy {
	rate := linearExploration
	if exploration > 0 {
		rate = exploration
	}
	policy := &LearnedPolicy{
		model:              model,
		guide:              NewHeuristic(guide, seed),
		random:             rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		sample:             sample,
		exploration:        rate,
		lastTradeRemaining: -1,
	}
	for decision := range LearnedDecisionCount {
		policy.expanded[decision] = model.usesResidual(decision)
	}
	return policy
}

func NewLearnedModel(weights [][]float64) (LearnedModel, error) {
	model := LearnedModel{}
	if len(weights) == 0 {
		return model, nil
	}
	if len(weights) != LearnedDecisionCount {
		return LearnedModel{}, fmt.Errorf("model has %d decisions, want %d", len(weights), LearnedDecisionCount)
	}
	for decision, row := range weights {
		if len(row) != LearnedParameterCount {
			return LearnedModel{}, fmt.Errorf("model decision %d has %d parameters, want %d", decision, len(row), LearnedParameterCount)
		}
		for parameter, weight := range row {
			if math.IsNaN(weight) || math.IsInf(weight, 0) {
				return LearnedModel{}, fmt.Errorf("model weight %d,%d is not finite", decision, parameter)
			}
			model.Weights[decision][parameter] = weight
		}
	}
	return model, nil
}

func (policy *LearnedPolicy) Turn(snapshot game.Snapshot) game.Command {
	guided := policy.guide.Turn(snapshot)
	candidates := markGuided(linearTurnCandidates(snapshot, policy.lastTradeRemaining), guided, linearTurnFeaturesFor(snapshot, guided))
	policy.enrich(linearTurnDecision, snapshot, candidates)
	command := policy.choose(linearTurnDecision, candidates).(game.Command)
	if _, trading := command.(game.BeginTrade); trading {
		policy.lastTradeRemaining = snapshot.Public.DeckRemaining
	}
	return command
}

func (policy *LearnedPolicy) Bid(snapshot game.Snapshot) (game.PlaceBid, bool) {
	bid, will := policy.guide.Bid(snapshot)
	guided := linearBid{bid: bid, will: will}
	features := linearFeatures{}
	if will {
		features = linearBidFeatures(snapshot, bid.Amount)
	}
	candidates := markGuided(linearBidCandidates(snapshot), guided, features)
	policy.enrich(linearBidDecision, snapshot, candidates)
	selected := policy.choose(linearBidDecision, candidates).(linearBid)
	return selected.bid, selected.will
}

func (policy *LearnedPolicy) ResolveAuction(snapshot game.Snapshot) game.ResolveAuction {
	guided := policy.guide.ResolveAuction(snapshot)
	features := linearFeatures{}
	if guided.Buy {
		features = linearBidFeatures(snapshot, moneyTotal(guided.Payment))
	}
	candidates := markGuided(linearResolveCandidates(snapshot), guided, features)
	policy.enrich(linearResolveDecision, snapshot, candidates)
	return policy.choose(linearResolveDecision, candidates).(game.ResolveAuction)
}

func (policy *LearnedPolicy) RespondTrade(snapshot game.Snapshot) game.Command {
	guided := policy.guide.RespondTrade(snapshot)
	features := linearFeatures{}
	if counter, ok := guided.(game.CounterTrade); ok {
		features = linearTradeFeatures(snapshot, counter.Offer)
	}
	candidates := markGuided(linearRespondCandidates(snapshot), guided, features)
	policy.enrich(linearRespondDecision, snapshot, candidates)
	return policy.choose(linearRespondDecision, candidates).(game.Command)
}

func (policy *LearnedPolicy) ReofferTrade(snapshot game.Snapshot) game.ReofferTrade {
	guided := policy.guide.ReofferTrade(snapshot)
	candidates := markGuided(linearReofferCandidates(snapshot), guided, linearTradeFeatures(snapshot, guided.Offer))
	policy.enrich(linearReofferDecision, snapshot, candidates)
	return policy.choose(linearReofferDecision, candidates).(game.ReofferTrade)
}

func (policy *LearnedPolicy) Observe(public game.PublicView) {
	policy.history.observe(public)
}

func (policy *LearnedPolicy) Gradient() *LearnedGradient {
	return &policy.gradient
}

func (policy *LearnedPolicy) Decisions() int {
	return policy.decisions
}

func (policy *LearnedPolicy) Deviations() [LearnedDecisionCount]int {
	return policy.deviations
}

func (policy *LearnedPolicy) enrich(decision int, snapshot game.Snapshot, candidates []linearCandidate) {
	for index := range candidates {
		features := &candidates[index].features
		interactions := linearInteractions(decision, *features)
		interactionEnd := linearBaseFeatureCount + linearInteractionFeatureCount
		copy(features[linearBaseFeatureCount:interactionEnd], interactions[:])
		history := policy.history.features(snapshot.Self.PlayerID, linearOtherPlayer(decision, snapshot, candidates[index].action))
		copy(features[interactionEnd:], history[:])
	}
}

func (policy *LearnedPolicy) choose(decision int, candidates []linearCandidate) any {
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
	anchor := guided
	for index := range probabilities {
		if candidates[index].expanded && !policy.expanded[decision] {
			continue
		}
		if probabilities[index] > probabilities[anchor] {
			anchor = index
		}
	}
	if probabilities[anchor] <= probabilities[guided]*math.Exp(linearEvalMargin) {
		anchor = guided
	}
	selected := anchor
	if policy.sample {
		policyProbabilities := make([]float64, len(candidates))
		for index, probability := range probabilities {
			policyProbabilities[index] = policy.exploration * probability / total
		}
		policyProbabilities[anchor] += 1 - policy.exploration
		draw := policy.random.Float64()
		selected = len(candidates) - 1
		for index, probability := range policyProbabilities {
			draw -= probability
			if draw <= 0 {
				selected = index
				break
			}
		}
		gradient := policy.scoreGradient(decision, candidates[selected].features)
		for index, candidate := range candidates {
			probability := probabilities[index] / total
			candidateGradient := policy.scoreGradient(decision, candidate.features)
			for parameter, value := range candidateGradient {
				gradient[parameter] -= probability * value
			}
		}
		scale := policy.exploration * probabilities[selected] / total / policyProbabilities[selected]
		for parameter := range gradient {
			policy.gradient[decision][parameter] += scale * gradient[parameter]
		}
		policy.decisions++
	}
	if selected != guided {
		policy.deviations[decision]++
	}
	return candidates[selected].action
}

func (model *LearnedModel) usesResidual(decision int) bool {
	for hidden := range LearnedHiddenCount {
		if model.Weights[decision][learnedOutputOffset(hidden)] != 0 {
			return true
		}
	}
	return false
}

func (policy *LearnedPolicy) probabilities(decision int, candidates []linearCandidate) ([]float64, float64) {
	scores := make([]float64, len(candidates))
	maximum := math.Inf(-1)
	for index, candidate := range candidates {
		scores[index] = policy.score(decision, candidate.features)
		maximum = max(maximum, scores[index])
	}
	total := 0.0
	for index, score := range scores {
		scores[index] = math.Exp(score - maximum)
		total += scores[index]
	}
	return scores, total
}

func (policy *LearnedPolicy) score(decision int, features linearFeatures) float64 {
	parameters := &policy.model.Weights[decision]
	score := 0.0
	for feature, value := range features {
		score += parameters[feature] * value
	}
	for hidden := range LearnedHiddenCount {
		score += parameters[learnedOutputOffset(hidden)] * policy.activation(parameters, hidden, features)
	}
	return score
}

func (policy *LearnedPolicy) scoreGradient(decision int, features linearFeatures) learnedParameters {
	parameters := &policy.model.Weights[decision]
	gradient := learnedParameters{}
	copy(gradient[:LearnedFeatureCount], features[:])
	for hidden := range LearnedHiddenCount {
		activation := policy.activation(parameters, hidden, features)
		output := parameters[learnedOutputOffset(hidden)]
		delta := output * (1 - activation*activation)
		for feature, value := range features {
			gradient[learnedHiddenOffset(hidden)+feature] = delta * value
		}
		gradient[learnedBiasOffset(hidden)] = delta
		gradient[learnedOutputOffset(hidden)] = activation
	}
	return gradient
}

func (policy *LearnedPolicy) activation(parameters *[LearnedParameterCount]float64, hidden int, features linearFeatures) float64 {
	value := parameters[learnedBiasOffset(hidden)]
	for feature, input := range features {
		value += parameters[learnedHiddenOffset(hidden)+feature] * input
	}
	return math.Tanh(value)
}

func learnedHiddenOffset(hidden int) int {
	return LearnedFeatureCount + hidden*LearnedFeatureCount
}

func learnedBiasOffset(hidden int) int {
	return LearnedFeatureCount + LearnedHiddenCount*LearnedFeatureCount + hidden
}

func learnedOutputOffset(hidden int) int {
	return LearnedFeatureCount + LearnedHiddenCount*(LearnedFeatureCount+1) + hidden
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
