package strategy

import "github.com/khoi/kuhhandel/internal/game"

func linearTurnFeaturesFor(snapshot game.Snapshot, command game.Command) linearFeatures {
	trade, ok := command.(game.BeginTrade)
	if !ok {
		return linearFeatures{}
	}
	for _, candidate := range linearTrades(snapshot) {
		if candidate.target.ID == trade.TargetID && candidate.animal == trade.Animal {
			return linearTurnFeatures(snapshot, candidate, trade.Offer)
		}
	}
	return linearFeatures{}
}

func linearTurnFeatures(snapshot game.Snapshot, trade linearTrade, offer game.Money) linearFeatures {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	ownCount := self.Animals[trade.animal]
	targetCount := trade.target.Animals[trade.animal]
	ownClaim := claimValue(self, trade.animal, trade.cards)
	targetClaim := claimValue(trade.target, trade.animal, trade.cards)
	money := float64(moneyTotal(snapshot.Self.Money))
	offerTotal := float64(moneyTotal(offer))
	return linearFeatures{
		1,
		1 - float64(snapshot.Public.DeckRemaining)/40,
		linearNormalize(money, 3000),
		float64(ownCount) / 4,
		float64(targetCount) / 4,
		float64(trade.cards) / 2,
		linearNormalize(ownClaim, 3000),
		linearNormalize(targetClaim, 3000),
		linearTruth(ownCount+trade.cards == 4),
		linearTruth(targetCount+trade.cards == 4),
		linearNormalize(float64(self.Score-trade.target.Score), 5000),
		linearNormalize(offerTotal, max(10, money)),
		linearNormalize(offerTotal, max(10, ownClaim+targetClaim)),
		linearTruth(offer.Zero > 0),
		float64(linearMoneyCardCount(offer)) / 10,
		float64(len(snapshot.Public.Players)) / 5,
	}
}

func linearBidFeatures(snapshot game.Snapshot, amount int) linearFeatures {
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	auction := snapshot.Public.Auction
	if auction == nil {
		return linearFeatures{}
	}
	ownClaim := claimValue(self, auction.Animal, 1)
	otherClaim := 0.0
	maxOtherCount := 0
	maxOtherScore := 0
	for _, player := range snapshot.Public.Players {
		if player.ID == self.ID {
			continue
		}
		otherClaim = max(otherClaim, claimValue(player, auction.Animal, 1))
		maxOtherCount = max(maxOtherCount, player.Animals[auction.Animal])
		maxOtherScore = max(maxOtherScore, player.Score)
	}
	money := float64(moneyTotal(snapshot.Self.Money))
	spend := float64(amount)
	return linearFeatures{
		1,
		linearNormalize(spend, max(10, money)),
		linearNormalize(spend, 3000),
		linearNormalize(money-spend, max(10, money)),
		linearNormalize(float64(auction.HighestBid), max(10, money)),
		linearNormalize(ownClaim, max(10, money)),
		linearNormalize(otherClaim, max(10, money)),
		float64(self.Animals[auction.Animal]) / 4,
		float64(maxOtherCount) / 4,
		linearTruth(self.Animals[auction.Animal] == 3),
		linearTruth(maxOtherCount == 3),
		1 - float64(snapshot.Public.DeckRemaining)/40,
		linearNormalize(float64(self.Score), 5000),
		linearNormalize(float64(self.Score-maxOtherScore), 5000),
		float64(len(snapshot.Public.Players)) / 5,
		linearNormalize(spend, max(10, ownClaim+otherClaim)),
	}
}

func linearTradeFeatures(snapshot game.Snapshot, offer game.Money) linearFeatures {
	trade := snapshot.Public.Trade
	if trade == nil {
		return linearFeatures{}
	}
	self := publicPlayer(snapshot.Public, snapshot.Self.PlayerID)
	otherID := trade.ChallengerID
	if otherID == self.ID {
		otherID = trade.TargetID
	}
	other := publicPlayer(snapshot.Public, otherID)
	ownClaim := claimValue(self, trade.Animal, trade.CardCount)
	otherClaim := claimValue(other, trade.Animal, trade.CardCount)
	money := float64(moneyTotal(snapshot.Self.Money))
	offerTotal := float64(moneyTotal(offer))
	return linearFeatures{
		1,
		linearNormalize(offerTotal, max(10, money)),
		linearNormalize(offerTotal, max(10, ownClaim+otherClaim)),
		linearTruth(offer.Zero > 0),
		float64(self.Animals[trade.Animal]) / 4,
		float64(other.Animals[trade.Animal]) / 4,
		float64(trade.CardCount) / 2,
		linearNormalize(ownClaim, 3000),
		linearNormalize(otherClaim, 3000),
		linearTruth(self.Animals[trade.Animal]+trade.CardCount == 4),
		linearTruth(other.Animals[trade.Animal]+trade.CardCount == 4),
		1 - float64(snapshot.Public.DeckRemaining)/40,
		linearTruth(snapshot.Public.Phase == game.PhaseTradeRecounter),
		linearNormalize(money, 3000),
		linearNormalize(float64(self.Score-other.Score), 5000),
		float64(linearMoneyCardCount(offer)) / 10,
	}
}

func linearNormalize(value, scale float64) float64 {
	return max(-2, min(2, value/scale))
}

func linearTruth(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

func linearMoneyCardCount(money game.Money) int {
	return money.Zero + money.Ten + money.Fifty + money.Hundred + money.TwoHundred + money.FiveHundred
}
