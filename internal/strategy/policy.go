package strategy

import (
	"math"
	"math/rand/v2"
	"sort"

	"github.com/khoi/kuhhandel/internal/game"
)

type Policy interface {
	Turn(game.Snapshot) game.Command
	Bid(game.Snapshot) (game.PlaceBid, bool)
	ResolveAuction(game.Snapshot) game.ResolveAuction
	RespondTrade(game.Snapshot) game.Command
	ReofferTrade(game.Snapshot) game.ReofferTrade
}

type HeuristicConfig struct {
	Name                 string
	AuctionFraction      float64
	DenialFraction       float64
	ReserveFraction      float64
	FirstRefusalFraction float64
	TradeFraction        float64
	TradeAtDeckRemaining int
	BluffChance          float64
}

func ThreePlayerChampion() HeuristicConfig {
	return HeuristicConfig{
		Name: "three-champion", AuctionFraction: 0.6, DenialFraction: 0.35, ReserveFraction: 0.3,
		FirstRefusalFraction: 0.25, TradeFraction: 0.75, TradeAtDeckRemaining: 0, BluffChance: 0,
	}
}

func LargeGameChampion() HeuristicConfig {
	threePlayer := ThreePlayerChampion()
	threePlayer.Name = "large-champion"
	threePlayer.DenialFraction = 0.65
	threePlayer.FirstRefusalFraction = 0.35
	threePlayer.TradeFraction = 1
	return threePlayer
}

type heuristic struct {
	config             HeuristicConfig
	random             *rand.Rand
	lastTradeRemaining int
}

func NewHeuristic(config HeuristicConfig, seed uint64) Policy {
	return &heuristic{
		config:             config,
		random:             rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)),
		lastTradeRemaining: -1,
	}
}

func (policy *heuristic) Turn(snapshot game.Snapshot) game.Command {
	trade, stake, completes := policy.bestTrade(snapshot)
	canAuction := hasAction(snapshot, "turn.auction")
	canTrade := hasAction(snapshot, "turn.trade")
	discretionaryTrade := snapshot.Public.DeckRemaining <= policy.config.TradeAtDeckRemaining && snapshot.Public.DeckRemaining != policy.lastTradeRemaining
	if canTrade && (!canAuction || completes || discretionaryTrade) {
		policy.lastTradeRemaining = snapshot.Public.DeckRemaining
		trade.Offer = policy.tradeOffer(snapshot.Self.Money, stake)
		return trade
	}
	return game.BeginAuction{}
}

func (policy *heuristic) Bid(snapshot game.Snapshot) (game.PlaceBid, bool) {
	auction := snapshot.Public.Auction
	if auction == nil {
		return game.PlaceBid{}, false
	}
	value := policy.auctionValue(snapshot, auction.Animal)
	cap := policy.spendCap(snapshot, value*policy.config.AuctionFraction)
	cap *= 0.9 + policy.random.Float64()*0.2
	minimum := auction.HighestBid + 1
	maximum := int(cap)
	target := minimum
	if gap := maximum - minimum; gap > 0 {
		target += policy.random.IntN(gap + 1)
	}
	payment, total, ok := paymentAtLeast(snapshot.Self.Money, target, maximum)
	if !ok {
		return game.PlaceBid{}, false
	}
	return game.PlaceBid{Amount: total, Payment: payment}, true
}

func (policy *heuristic) ResolveAuction(snapshot game.Snapshot) game.ResolveAuction {
	auction := snapshot.Public.Auction
	if auction == nil {
		return game.ResolveAuction{}
	}
	value := policy.auctionValue(snapshot, auction.Animal)
	cap := policy.spendCap(snapshot, value*policy.config.FirstRefusalFraction)
	payment, _, ok := paymentAtLeast(snapshot.Self.Money, auction.HighestBid, int(cap))
	if !ok {
		return game.ResolveAuction{}
	}
	return game.ResolveAuction{Buy: true, Payment: payment}
}

func (policy *heuristic) RespondTrade(snapshot game.Snapshot) game.Command {
	stake := policy.tradeStake(snapshot)
	if snapshot.Public.Phase == game.PhaseTradeResponse && snapshot.Self.Money.Zero > 0 && policy.random.Float64() < policy.config.BluffChance {
		return game.CounterTrade{Offer: game.Money{Zero: 1}}
	}
	offer, ok := paymentAtMost(snapshot.Self.Money, policy.randomizedTradeCap(stake))
	if !ok {
		return game.CounterTrade{Offer: smallestOffer(snapshot.Self.Money)}
	}
	return game.CounterTrade{Offer: offer}
}

func (policy *heuristic) ReofferTrade(snapshot game.Snapshot) game.ReofferTrade {
	stake := policy.tradeStake(snapshot)
	return game.ReofferTrade{Offer: policy.tradeOffer(snapshot.Self.Money, stake)}
}

func (policy *heuristic) bestTrade(snapshot game.Snapshot) (game.BeginTrade, float64, bool) {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	best := game.BeginTrade{}
	bestStake := -1.0
	bestPriority := -1
	completes := false
	animals := make([]game.Animal, 0, len(self.Animals))
	for animal, count := range self.Animals {
		if count > 0 && count < 4 {
			animals = append(animals, animal)
		}
	}
	sort.Slice(animals, func(first, second int) bool {
		return animals[first] < animals[second]
	})
	for _, animal := range animals {
		ownCount := self.Animals[animal]
		for _, target := range snapshot.Public.Players {
			targetCount := target.Animals[animal]
			if target.ID == self.ID || targetCount == 0 || targetCount == 4 {
				continue
			}
			cardCount := 1
			if ownCount == 2 && targetCount == 2 {
				cardCount = 2
			}
			stake := claimValue(self, animal, cardCount) + policy.config.DenialFraction*claimValue(target, animal, cardCount)
			priority := 0
			if snapshot.Public.DeckRemaining == 0 {
				priority = tradePriority(ownCount, targetCount, cardCount)
			}
			if priority > bestPriority || priority == bestPriority && stake > bestStake {
				best = game.BeginTrade{TargetID: target.ID, Animal: animal}
				bestStake = stake
				bestPriority = priority
				completes = ownCount+cardCount == 4
			}
		}
	}
	return best, bestStake, completes
}

func tradePriority(ownCount, targetCount, cardCount int) int {
	switch {
	case ownCount+cardCount == 4:
		return 4
	case ownCount == targetCount:
		return 3
	case targetCount+cardCount == 4:
		return 2
	default:
		return 1
	}
}

func (policy *heuristic) auctionValue(snapshot game.Snapshot, animal game.Animal) float64 {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	value := claimValue(self, animal, 1)
	denial := 0.0
	for _, player := range snapshot.Public.Players {
		if player.ID != self.ID {
			denial = math.Max(denial, claimValue(player, animal, 1))
		}
	}
	return value + policy.config.DenialFraction*denial
}

func (policy *heuristic) tradeStake(snapshot game.Snapshot) float64 {
	trade := snapshot.Public.Trade
	if trade == nil {
		return 0
	}
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	otherID := trade.ChallengerID
	if otherID == self.ID {
		otherID = trade.TargetID
	}
	other := publicPlayer(snapshot.Public, otherID)
	return claimValue(self, trade.Animal, trade.CardCount) + policy.config.DenialFraction*claimValue(other, trade.Animal, trade.CardCount)
}

func (policy *heuristic) spendCap(snapshot game.Snapshot, value float64) float64 {
	total := float64(moneyTotal(snapshot.Self.Money))
	available := total * (1 - policy.config.ReserveFraction)
	return math.Min(value, available)
}

func (policy *heuristic) tradeOffer(money game.Money, stake float64) game.Money {
	if money.Zero > 0 && policy.random.Float64() < policy.config.BluffChance {
		return game.Money{Zero: 1}
	}
	offer, ok := paymentAtMost(money, policy.randomizedTradeCap(stake))
	if ok {
		return offer
	}
	return smallestOffer(money)
}

func (policy *heuristic) randomizedTradeCap(stake float64) int {
	fraction := 0.5 + policy.random.Float64()*0.5
	return int(stake * policy.config.TradeFraction * fraction)
}

func claimValue(player game.PublicPlayer, animal game.Animal, cards int) float64 {
	count := player.Animals[animal]
	remaining := 4 - count
	if count == 4 || cards <= 0 || remaining <= 0 {
		return 0
	}
	if cards > remaining {
		cards = remaining
	}
	animals := make(map[game.Animal]int, len(player.Animals)+1)
	for owned, count := range player.Animals {
		animals[owned] = count
	}
	before := game.Score(animals)
	animals[animal] = 4
	return float64(game.Score(animals)-before) * float64(cards) / float64(remaining)
}

func publicPlayer(public game.PublicView, playerID string) game.PublicPlayer {
	for _, player := range public.Players {
		if player.ID == playerID {
			return player
		}
	}
	return game.PublicPlayer{}
}

func hasAction(snapshot game.Snapshot, action string) bool {
	for _, legal := range snapshot.Self.LegalActions {
		if legal == action {
			return true
		}
	}
	return false
}
