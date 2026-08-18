package strategy

import "github.com/khoi/kuhhandel/internal/game"

type linearHistory struct {
	players map[string]linearPlayerHistory
	auction *linearAuctionHistory
	trade   *linearTradeHistory
	donkeys int
}

type linearPlayerHistory struct {
	money       int
	bids        int
	bidTotal    int
	maxBid      int
	auctionWins int
	trades      int
	tradeWins   int
}

type linearAuctionHistory struct {
	animal     game.Animal
	auctioneer string
	bidder     string
	amount     int
	animals    map[string]int
}

type linearTradeHistory struct {
	animal  game.Animal
	animals map[string]int
}

func (history *linearHistory) observe(public game.PublicView) {
	if history.players == nil {
		history.players = make(map[string]linearPlayerHistory, len(public.Players))
		for _, player := range public.Players {
			history.players[player.ID] = linearPlayerHistory{money: 90}
		}
	}
	history.observeAuction(public)
	history.observeTrade(public)
}

func (history *linearHistory) observeAuction(public game.PublicView) {
	if public.Auction == nil {
		if history.auction != nil {
			history.settleAuction(public)
			history.auction = nil
		}
		return
	}
	if history.auction == nil {
		history.auction = &linearAuctionHistory{
			animal:     public.Auction.Animal,
			auctioneer: public.Auction.AuctioneerID,
			animals:    animalCounts(public.Players, public.Auction.Animal),
		}
		if public.Auction.Animal == game.AnimalDonkey {
			history.donkeys++
			bonus := [...]int{0, 50, 100, 200, 500}[history.donkeys]
			for playerID, player := range history.players {
				player.money += bonus
				history.players[playerID] = player
			}
		}
	}
	if public.Auction.HighestBid <= history.auction.amount {
		return
	}
	player := history.players[public.Auction.HighestBidderID]
	player.bids++
	player.bidTotal += public.Auction.HighestBid
	player.maxBid = max(player.maxBid, public.Auction.HighestBid)
	history.players[public.Auction.HighestBidderID] = player
	history.auction.bidder = public.Auction.HighestBidderID
	history.auction.amount = public.Auction.HighestBid
}

func (history *linearHistory) settleAuction(public game.PublicView) {
	winner := changedAnimalOwner(public.Players, history.auction.animal, history.auction.animals)
	winnerHistory := history.players[winner]
	winnerHistory.auctionWins++
	history.players[winner] = winnerHistory
	if history.auction.bidder == "" {
		return
	}
	payer := history.auction.bidder
	receiver := history.auction.auctioneer
	if winner == history.auction.auctioneer {
		payer, receiver = receiver, payer
	}
	payerHistory := history.players[payer]
	payerHistory.money -= history.auction.amount
	history.players[payer] = payerHistory
	receiverHistory := history.players[receiver]
	receiverHistory.money += history.auction.amount
	history.players[receiver] = receiverHistory
}

func (history *linearHistory) observeTrade(public game.PublicView) {
	if public.Trade == nil {
		if history.trade != nil {
			winner := changedAnimalOwner(public.Players, history.trade.animal, history.trade.animals)
			winnerHistory := history.players[winner]
			winnerHistory.tradeWins++
			history.players[winner] = winnerHistory
			history.trade = nil
		}
		return
	}
	if history.trade != nil {
		return
	}
	history.trade = &linearTradeHistory{
		animal:  public.Trade.Animal,
		animals: animalCounts(public.Players, public.Trade.Animal),
	}
	challenger := history.players[public.Trade.ChallengerID]
	challenger.trades++
	history.players[public.Trade.ChallengerID] = challenger
}

func (history *linearHistory) features(selfID, otherID string) [8]float64 {
	if otherID == "" {
		otherID = history.richestOpponent(selfID)
	}
	self := history.players[selfID]
	other := history.players[otherID]
	meanBid := 0.0
	if other.bids > 0 {
		meanBid = float64(other.bidTotal) / float64(other.bids)
	}
	return [8]float64{
		linearNormalize(float64(other.money), 3000),
		linearNormalize(float64(self.money-other.money), 3000),
		linearNormalize(float64(other.maxBid), 3000),
		linearNormalize(meanBid, 3000),
		linearNormalize(float64(other.bids), 40),
		linearNormalize(float64(other.auctionWins), 10),
		linearNormalize(float64(other.trades), 20),
		linearNormalize(float64(other.tradeWins), 10),
	}
}

func (history *linearHistory) richestOpponent(selfID string) string {
	selected := ""
	maximum := -1
	for playerID, player := range history.players {
		if playerID != selfID && (player.money > maximum || player.money == maximum && playerID < selected) {
			selected = playerID
			maximum = player.money
		}
	}
	return selected
}

func animalCounts(players []game.PublicPlayer, animal game.Animal) map[string]int {
	counts := make(map[string]int, len(players))
	for _, player := range players {
		counts[player.ID] = player.Animals[animal]
	}
	return counts
}

func changedAnimalOwner(players []game.PublicPlayer, animal game.Animal, before map[string]int) string {
	for _, player := range players {
		if player.Animals[animal] > before[player.ID] {
			return player.ID
		}
	}
	return ""
}
