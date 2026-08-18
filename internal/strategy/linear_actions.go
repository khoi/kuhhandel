package strategy

import (
	"sort"

	"github.com/khoi/kuhhandel/internal/game"
)

type linearTrade struct {
	target game.PublicPlayer
	animal game.Animal
	cards  int
}

func linearTurnCandidates(snapshot game.Snapshot, lastTradeRemaining int) []linearCandidate {
	candidates := []linearCandidate{}
	canAuction := hasAction(snapshot, "turn.auction")
	if canAuction {
		candidates = append(candidates, linearCandidate{action: game.Command(game.BeginAuction{})})
	}
	if !hasAction(snapshot, "turn.trade") || canAuction && snapshot.Public.DeckRemaining == lastTradeRemaining {
		return candidates
	}
	trades := linearTrades(snapshot)
	if snapshot.Public.DeckRemaining == 0 {
		priority := 0
		for _, trade := range trades {
			priority = max(priority, linearTradePriority(snapshot, trade))
		}
		selected := trades[:0]
		for _, trade := range trades {
			if linearTradePriority(snapshot, trade) == priority {
				selected = append(selected, trade)
			}
		}
		trades = selected
	}
	for _, trade := range trades {
		for _, offer := range linearOffers(snapshot.Self.Money) {
			command := game.BeginTrade{TargetID: trade.target.ID, Animal: trade.animal, Offer: offer}
			candidates = append(candidates, linearCandidate{action: game.Command(command), features: linearTurnFeatures(snapshot, trade, offer)})
		}
	}
	return candidates
}

func linearBidCandidates(snapshot game.Snapshot) []linearCandidate {
	candidates := []linearCandidate{{action: linearBid{}}}
	if snapshot.Public.Auction == nil {
		return candidates
	}
	minimum := snapshot.Public.Auction.HighestBid + 1
	for _, option := range linearBidOptions(snapshot.Self.Money, minimum) {
		bid := game.PlaceBid{Amount: option.total, Payment: option.money}
		candidates = append(candidates, linearCandidate{action: linearBid{bid: bid, will: true}, features: linearBidFeatures(snapshot, option.total)})
	}
	return candidates
}

func linearResolveCandidates(snapshot game.Snapshot) []linearCandidate {
	candidates := []linearCandidate{{action: game.ResolveAuction{}}}
	if snapshot.Public.Auction == nil {
		return candidates
	}
	maximum := moneyTotal(snapshot.Self.Money)
	payment, total, ok := paymentAtLeast(snapshot.Self.Money, snapshot.Public.Auction.HighestBid, maximum)
	if ok {
		resolution := game.ResolveAuction{Buy: true, Payment: payment}
		candidates = append(candidates, linearCandidate{action: resolution, features: linearBidFeatures(snapshot, total)})
	}
	return candidates
}

func linearRespondCandidates(snapshot game.Snapshot) []linearCandidate {
	candidates := []linearCandidate{}
	if snapshot.Public.Phase == game.PhaseTradeResponse {
		candidates = append(candidates, linearCandidate{action: game.Command(game.AcceptTrade{})})
	}
	for _, offer := range linearOffers(snapshot.Self.Money) {
		command := game.CounterTrade{Offer: offer}
		candidates = append(candidates, linearCandidate{action: game.Command(command), features: linearTradeFeatures(snapshot, offer)})
	}
	return candidates
}

func linearReofferCandidates(snapshot game.Snapshot) []linearCandidate {
	candidates := []linearCandidate{}
	for _, offer := range linearOffers(snapshot.Self.Money) {
		command := game.ReofferTrade{Offer: offer}
		candidates = append(candidates, linearCandidate{action: command, features: linearTradeFeatures(snapshot, offer)})
	}
	return candidates
}

func linearTrades(snapshot game.Snapshot) []linearTrade {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	animals := make([]game.Animal, 0, len(self.Animals))
	for animal, count := range self.Animals {
		if count > 0 && count < 4 {
			animals = append(animals, animal)
		}
	}
	sort.Slice(animals, func(first, second int) bool {
		return animals[first] < animals[second]
	})
	trades := []linearTrade{}
	for _, animal := range animals {
		for _, target := range snapshot.Public.Players {
			targetCount := target.Animals[animal]
			if target.ID == self.ID || targetCount == 0 || targetCount == 4 {
				continue
			}
			cards := 1
			if self.Animals[animal] == 2 && targetCount == 2 {
				cards = 2
			}
			trades = append(trades, linearTrade{target: target, animal: animal, cards: cards})
		}
	}
	return trades
}

func linearTradePriority(snapshot game.Snapshot, trade linearTrade) int {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	return tradePriority(self.Animals[trade.animal], trade.target.Animals[trade.animal], trade.cards)
}

func linearOffers(money game.Money) []game.Money {
	offers := []game.Money{}
	seen := map[game.Money]bool{}
	add := func(offer game.Money) {
		if !seen[offer] {
			seen[offer] = true
			offers = append(offers, offer)
		}
	}
	if money.Zero > 0 {
		add(game.Money{Zero: 1})
	}
	options := paymentOptions(money)
	total := moneyTotal(money)
	for _, fraction := range [...]float64{0.1, 0.25, 0.5, 0.75, 1} {
		maximum := int(float64(total) * fraction)
		for index := len(options) - 1; index >= 0; index-- {
			if options[index].total <= maximum {
				add(options[index].money)
				break
			}
		}
	}
	return offers
}

func linearBidOptions(money game.Money, minimum int) []paymentOption {
	options := paymentOptions(money)
	selected := []paymentOption{}
	seen := map[int]bool{}
	add := func(option paymentOption) {
		if option.total >= minimum && !seen[option.total] {
			seen[option.total] = true
			selected = append(selected, option)
		}
	}
	for _, option := range options {
		if option.total >= minimum {
			add(option)
			break
		}
	}
	total := moneyTotal(money)
	for _, fraction := range [...]float64{0.25, 0.5, 0.75, 1} {
		maximum := int(float64(total) * fraction)
		for index := len(options) - 1; index >= 0; index-- {
			if options[index].total <= maximum {
				add(options[index])
				break
			}
		}
	}
	sort.Slice(selected, func(first, second int) bool {
		return selected[first].total < selected[second].total
	})
	return selected
}
