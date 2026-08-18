package game

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Status string

const (
	StatusLobby    Status = "lobby"
	StatusPlaying  Status = "playing"
	StatusFinished Status = "finished"
)

type Phase string

const (
	PhaseLobby          Phase = "lobby"
	PhaseTurn           Phase = "turn"
	PhaseAuction        Phase = "auction"
	PhaseFirstRefusal   Phase = "first_refusal"
	PhaseTradeResponse  Phase = "trade_response"
	PhaseTradeReoffer   Phase = "trade_reoffer"
	PhaseTradeRecounter Phase = "trade_recounter"
	PhaseFinished       Phase = "finished"
)

type Identity struct {
	ID       string `json:"id"`
	AuthHash string `json:"authHash"`
	Name     string `json:"name"`
}

type Command interface {
	command()
}

type CreateRoom struct {
	Player Identity
}

func (CreateRoom) command() {}

type JoinRoom struct {
	Player Identity
}

func (JoinRoom) command() {}

type StartGame struct {
	Seed uint64
}

func (StartGame) command() {}

type BeginAuction struct{}

func (BeginAuction) command() {}

type CloseAuction struct{}

func (CloseAuction) command() {}

type PlaceBid struct {
	Amount  int
	Payment Money
}

func (PlaceBid) command() {}

type ResolveAuction struct {
	Buy     bool
	Payment Money
}

func (ResolveAuction) command() {}

type BeginTrade struct {
	TargetID string
	Animal   Animal
	Offer    Money
}

func (BeginTrade) command() {}

type AcceptTrade struct{}

func (AcceptTrade) command() {}

type CounterTrade struct {
	Offer Money
}

func (CounterTrade) command() {}

type ReofferTrade struct {
	Offer Money
}

func (ReofferTrade) command() {}

type Event struct {
	Version    uint64          `json:"version"`
	Type       string          `json:"type"`
	Data       json.RawMessage `json:"data"`
	ActorID    string          `json:"actorId,omitempty"`
	CommandID  string          `json:"commandId,omitempty"`
	OccurredAt time.Time       `json:"occurredAt,omitempty"`
}

type RuleError struct {
	Code    string
	Message string
}

func (e *RuleError) Error() string {
	return e.Message
}

type Aggregate struct {
	state state
}

type state struct {
	ID      string
	Version uint64
	Status  Status
	Phase   Phase
	HostID  string
	Players []player
	Turn    int
	Deck    []Animal
	DeckPos int
	Bank    Money
	Auction *auction
	Donkeys int
	Trade   *trade
}

type player struct {
	Identity
	Money   Money
	Animals map[Animal]int
}

type auction struct {
	Animal          Animal
	AuctioneerID    string
	HighestBid      int
	HighestBidderID string
	Payment         Money
}

type trade struct {
	ChallengerID    string
	TargetID        string
	Animal          Animal
	CardCount       int
	ChallengerOffer Money
	Round           int
}

type Snapshot struct {
	Version uint64     `json:"version"`
	Public  PublicView `json:"public"`
	Self    SelfView   `json:"self"`
}

type PublicView struct {
	GameID        string         `json:"gameId"`
	Status        Status         `json:"status"`
	Phase         Phase          `json:"phase"`
	HostID        string         `json:"hostId"`
	TurnPlayerID  string         `json:"turnPlayerId,omitempty"`
	Players       []PublicPlayer `json:"players"`
	DeckRemaining int            `json:"deckRemaining"`
	Auction       *AuctionView   `json:"auction,omitempty"`
	Trade         *TradeView     `json:"trade,omitempty"`
	WinnerIDs     []string       `json:"winnerIds,omitempty"`
}

type PublicPlayer struct {
	ID      string         `json:"id"`
	Name    string         `json:"name"`
	Seat    int            `json:"seat"`
	Animals map[Animal]int `json:"animals"`
	Score   int            `json:"score"`
}

type AuctionView struct {
	Animal          Animal `json:"animal"`
	AuctioneerID    string `json:"auctioneerId"`
	HighestBid      int    `json:"highestBid"`
	HighestBidderID string `json:"highestBidderId,omitempty"`
}

type TradeView struct {
	ChallengerID string `json:"challengerId"`
	TargetID     string `json:"targetId"`
	Animal       Animal `json:"animal"`
	CardCount    int    `json:"cardCount"`
}

type SelfView struct {
	PlayerID     string   `json:"playerId"`
	Money        Money    `json:"money"`
	LegalActions []string `json:"legalActions"`
	BidPayment   *Money   `json:"bidPayment,omitempty"`
	OwnOffer     *Money   `json:"ownOffer,omitempty"`
}

type roomCreated struct {
	Player Identity `json:"player"`
}

type playerJoined struct {
	Player Identity `json:"player"`
}

type gameStarted struct {
	Seed uint64 `json:"seed"`
}

type bidPlaced struct {
	PlayerID string `json:"playerId"`
	Amount   int    `json:"amount"`
	Payment  Money  `json:"payment"`
}

type auctionResolved struct {
	Buy     bool  `json:"buy"`
	Payment Money `json:"payment"`
}

type tradeStarted struct {
	ChallengerID string `json:"challengerId"`
	TargetID     string `json:"targetId"`
	Animal       Animal `json:"animal"`
	CardCount    int    `json:"cardCount"`
	Offer        Money  `json:"offer"`
}

type tradeCountered struct {
	Offer Money `json:"offer"`
}

type tradeReoffered struct {
	Offer Money `json:"offer"`
}

func New(id string) *Aggregate {
	return &Aggregate{state: state{ID: id}}
}

func (a *Aggregate) Decide(actorID string, command Command) (Event, error) {
	switch command := command.(type) {
	case CreateRoom:
		if a.state.Version != 0 {
			return Event{}, ruleError("invalid_phase", "room already exists")
		}
		if err := validateIdentity(command.Player); err != nil {
			return Event{}, err
		}
		return newEvent(a.state.Version+1, "room.created", roomCreated{Player: command.Player})
	case JoinRoom:
		if a.state.Status != StatusLobby {
			return Event{}, ruleError("invalid_phase", "room is not accepting players")
		}
		if len(a.state.Players) >= 5 {
			return Event{}, ruleError("room_full", "room already has five players")
		}
		if err := validateIdentity(command.Player); err != nil {
			return Event{}, err
		}
		for _, player := range a.state.Players {
			if player.ID == command.Player.ID || player.AuthHash == command.Player.AuthHash {
				return Event{}, ruleError("player_exists", "player already joined")
			}
		}
		return newEvent(a.state.Version+1, "player.joined", playerJoined{Player: command.Player})
	case StartGame:
		if a.state.Status != StatusLobby {
			return Event{}, ruleError("invalid_phase", "game already started")
		}
		if actorID != a.state.HostID {
			return Event{}, ruleError("forbidden", "only the host can start the game")
		}
		if len(a.state.Players) < 3 {
			return Event{}, ruleError("not_enough_players", "at least three players are required")
		}
		return newEvent(a.state.Version+1, "game.started", gameStarted{Seed: command.Seed})
	case BeginAuction:
		if err := a.requireTurn(actorID); err != nil {
			return Event{}, err
		}
		if a.state.DeckPos >= len(a.state.Deck) {
			return Event{}, ruleError("deck_empty", "no animals remain for auction")
		}
		return newEvent(a.state.Version+1, "auction.started", struct{}{})
	case CloseAuction:
		if a.state.Phase != PhaseAuction || a.state.Auction == nil {
			return Event{}, ruleError("invalid_phase", "there is no auction to close")
		}
		if actorID != a.state.Auction.AuctioneerID {
			return Event{}, ruleError("forbidden", "only the auctioneer can close the auction")
		}
		return newEvent(a.state.Version+1, "auction.closed", struct{}{})
	case PlaceBid:
		if a.state.Phase != PhaseAuction || a.state.Auction == nil {
			return Event{}, ruleError("invalid_phase", "there is no open auction")
		}
		if actorID == a.state.Auction.AuctioneerID {
			return Event{}, ruleError("forbidden", "auctioneer cannot bid")
		}
		if command.Amount <= a.state.Auction.HighestBid {
			return Event{}, ruleError("bid_too_low", "bid must exceed the current bid")
		}
		playerIndex := a.playerIndex(actorID)
		if playerIndex == -1 {
			return Event{}, ruleError("not_a_player", "player is not in this game")
		}
		if err := validateAuctionPayment(a.state.Players[playerIndex].Money, command.Amount, command.Payment); err != nil {
			return Event{}, err
		}
		return newEvent(a.state.Version+1, "auction.bid_placed", bidPlaced{PlayerID: actorID, Amount: command.Amount, Payment: command.Payment})
	case ResolveAuction:
		if a.state.Phase != PhaseFirstRefusal || a.state.Auction == nil {
			return Event{}, ruleError("invalid_phase", "auction is not awaiting first refusal")
		}
		if actorID != a.state.Auction.AuctioneerID {
			return Event{}, ruleError("forbidden", "only the auctioneer can resolve first refusal")
		}
		if !command.Buy && command.Payment != (Money{}) {
			return Event{}, ruleError("invalid_payment", "declining an auction does not use payment")
		}
		if command.Buy {
			playerIndex := a.playerIndex(actorID)
			if err := validateAuctionPayment(a.state.Players[playerIndex].Money, a.state.Auction.HighestBid, command.Payment); err != nil {
				return Event{}, err
			}
		}
		return newEvent(a.state.Version+1, "auction.resolved", auctionResolved{Buy: command.Buy, Payment: command.Payment})
	case BeginTrade:
		if err := a.requireTurn(actorID); err != nil {
			return Event{}, err
		}
		challenger := a.playerIndex(actorID)
		target := a.playerIndex(command.TargetID)
		if target == -1 || target == challenger {
			return Event{}, ruleError("invalid_target", "trade target must be another player")
		}
		challengerCount := a.state.Players[challenger].Animals[command.Animal]
		targetCount := a.state.Players[target].Animals[command.Animal]
		if challengerCount == 0 || challengerCount == 4 || targetCount == 0 || targetCount == 4 {
			return Event{}, ruleError("invalid_trade", "players do not share a tradable animal")
		}
		if !command.Offer.valid() || !a.state.Players[challenger].Money.contains(command.Offer) || command.Offer.cardCount() == 0 {
			return Event{}, ruleError("invalid_payment", "challenger does not own the offered cards")
		}
		cardCount := 1
		if challengerCount == 2 && targetCount == 2 {
			cardCount = 2
		}
		return newEvent(a.state.Version+1, "trade.started", tradeStarted{
			ChallengerID: actorID,
			TargetID:     command.TargetID,
			Animal:       command.Animal,
			CardCount:    cardCount,
			Offer:        command.Offer,
		})
	case AcceptTrade:
		if a.state.Phase != PhaseTradeResponse || a.state.Trade == nil {
			return Event{}, ruleError("invalid_phase", "there is no trade to accept")
		}
		if actorID != a.state.Trade.TargetID {
			return Event{}, ruleError("forbidden", "only the target can accept the trade")
		}
		return newEvent(a.state.Version+1, "trade.accepted", struct{}{})
	case CounterTrade:
		if (a.state.Phase != PhaseTradeResponse && a.state.Phase != PhaseTradeRecounter) || a.state.Trade == nil {
			return Event{}, ruleError("invalid_phase", "there is no trade to counter")
		}
		if actorID != a.state.Trade.TargetID {
			return Event{}, ruleError("forbidden", "only the target can counter the trade")
		}
		target := a.playerIndex(actorID)
		if !command.Offer.valid() || !a.state.Players[target].Money.contains(command.Offer) || command.Offer.cardCount() == 0 {
			return Event{}, ruleError("invalid_payment", "target does not own the offered cards")
		}
		return newEvent(a.state.Version+1, "trade.countered", tradeCountered{Offer: command.Offer})
	case ReofferTrade:
		if a.state.Phase != PhaseTradeReoffer || a.state.Trade == nil {
			return Event{}, ruleError("invalid_phase", "trade is not awaiting a second offer")
		}
		if actorID != a.state.Trade.ChallengerID {
			return Event{}, ruleError("forbidden", "only the challenger can make the second offer")
		}
		challenger := a.playerIndex(actorID)
		if !command.Offer.valid() || !a.state.Players[challenger].Money.contains(command.Offer) || command.Offer.cardCount() == 0 {
			return Event{}, ruleError("invalid_payment", "challenger does not own the offered cards")
		}
		return newEvent(a.state.Version+1, "trade.reoffered", tradeReoffered{Offer: command.Offer})
	default:
		return Event{}, ruleError("unknown_command", "unknown command")
	}
}

func (a *Aggregate) Apply(event Event) error {
	if event.Version != a.state.Version+1 {
		return fmt.Errorf("event version %d follows %d", event.Version, a.state.Version)
	}
	switch event.Type {
	case "room.created":
		var data roomCreated
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Status = StatusLobby
		a.state.Phase = PhaseLobby
		a.state.HostID = data.Player.ID
		a.state.Players = []player{{Identity: data.Player, Animals: map[Animal]int{}}}
	case "player.joined":
		var data playerJoined
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Players = append(a.state.Players, player{Identity: data.Player, Animals: map[Animal]int{}})
	case "game.started":
		var data gameStarted
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Status = StatusPlaying
		a.state.Phase = PhaseTurn
		a.state.Turn = 0
		a.state.Deck = shuffledDeck(data.Seed)
		a.state.DeckPos = 0
		a.state.Bank = Money{Zero: 10, Ten: 20, Fifty: 10, Hundred: 5, TwoHundred: 5, FiveHundred: 5}
		startingMoney := Money{Zero: 2, Ten: 4, Fifty: 1}
		for index := range a.state.Players {
			bank, err := a.state.Bank.subtract(startingMoney)
			if err != nil {
				return err
			}
			a.state.Bank = bank
			a.state.Players[index].Money = startingMoney
		}
	case "auction.started":
		animal := a.state.Deck[a.state.DeckPos]
		a.state.DeckPos++
		if animal == AnimalDonkey {
			a.state.Donkeys++
			bonus := donkeyBonus(a.state.Donkeys)
			for index := range a.state.Players {
				bank, err := a.state.Bank.subtract(bonus)
				if err != nil {
					return err
				}
				a.state.Bank = bank
				a.state.Players[index].Money = a.state.Players[index].Money.add(bonus)
			}
		}
		a.state.Phase = PhaseAuction
		a.state.Auction = &auction{Animal: animal, AuctioneerID: a.state.Players[a.state.Turn].ID}
	case "auction.closed":
		if a.state.Auction.HighestBidderID == "" {
			a.state.Players[a.state.Turn].Animals[a.state.Auction.Animal]++
			a.advanceTurn()
		} else {
			a.state.Phase = PhaseFirstRefusal
		}
	case "auction.bid_placed":
		var data bidPlaced
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Auction.HighestBid = data.Amount
		a.state.Auction.HighestBidderID = data.PlayerID
		a.state.Auction.Payment = data.Payment
	case "auction.resolved":
		var data auctionResolved
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		auctioneer := a.playerIndex(a.state.Auction.AuctioneerID)
		bidder := a.playerIndex(a.state.Auction.HighestBidderID)
		if data.Buy {
			if err := a.transferMoney(auctioneer, bidder, data.Payment); err != nil {
				return err
			}
			a.state.Players[auctioneer].Animals[a.state.Auction.Animal]++
		} else {
			if err := a.transferMoney(bidder, auctioneer, a.state.Auction.Payment); err != nil {
				return err
			}
			a.state.Players[bidder].Animals[a.state.Auction.Animal]++
		}
		a.advanceTurn()
	case "trade.started":
		var data tradeStarted
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Trade = &trade{
			ChallengerID:    data.ChallengerID,
			TargetID:        data.TargetID,
			Animal:          data.Animal,
			CardCount:       data.CardCount,
			ChallengerOffer: data.Offer,
		}
		a.state.Phase = PhaseTradeResponse
	case "trade.accepted":
		challenger := a.playerIndex(a.state.Trade.ChallengerID)
		target := a.playerIndex(a.state.Trade.TargetID)
		if err := a.transferMoney(challenger, target, a.state.Trade.ChallengerOffer.withoutZero()); err != nil {
			return err
		}
		if err := a.transferAnimals(target, challenger, a.state.Trade.Animal, a.state.Trade.CardCount); err != nil {
			return err
		}
		a.advanceTurn()
	case "trade.countered":
		var data tradeCountered
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		challenger := a.playerIndex(a.state.Trade.ChallengerID)
		target := a.playerIndex(a.state.Trade.TargetID)
		if err := a.exchangeMoney(challenger, a.state.Trade.ChallengerOffer, target, data.Offer); err != nil {
			return err
		}
		challengerTotal := a.state.Trade.ChallengerOffer.total()
		targetTotal := data.Offer.total()
		switch {
		case challengerTotal > targetTotal:
			if err := a.transferAnimals(target, challenger, a.state.Trade.Animal, a.state.Trade.CardCount); err != nil {
				return err
			}
			a.advanceTurn()
		case targetTotal > challengerTotal:
			if err := a.transferAnimals(challenger, target, a.state.Trade.Animal, a.state.Trade.CardCount); err != nil {
				return err
			}
			a.advanceTurn()
		default:
			if a.state.Trade.Round == 2 {
				if err := a.transferAnimals(target, challenger, a.state.Trade.Animal, a.state.Trade.CardCount); err != nil {
					return err
				}
				a.advanceTurn()
			} else {
				a.state.Trade.ChallengerOffer = Money{}
				a.state.Trade.Round = 2
				a.state.Phase = PhaseTradeReoffer
			}
		}
	case "trade.reoffered":
		var data tradeReoffered
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return err
		}
		a.state.Trade.ChallengerOffer = data.Offer
		a.state.Phase = PhaseTradeRecounter
	default:
		return fmt.Errorf("unknown event %q", event.Type)
	}
	a.state.Version = event.Version
	return nil
}

func (a *Aggregate) Snapshot(playerID string) (Snapshot, error) {
	public, self := a.publicView(playerID)
	if self == -1 {
		return Snapshot{}, ruleError("not_a_player", "player is not in this game")
	}
	legalActions := []string{}
	var bidPayment *Money
	var ownOffer *Money
	if a.state.Status == StatusPlaying && len(a.state.Players) > 0 {
		if public.TurnPlayerID == playerID && a.state.Phase == PhaseTurn && a.state.DeckPos < len(a.state.Deck) {
			legalActions = append(legalActions, "turn.auction")
		}
		if public.TurnPlayerID == playerID && a.state.Phase == PhaseTurn && a.canTrade(self) {
			legalActions = append(legalActions, "turn.trade")
		}
		if a.state.Phase == PhaseAuction && a.state.Auction != nil && a.state.Auction.AuctioneerID == playerID {
			legalActions = append(legalActions, "auction.close")
		}
		if a.state.Phase == PhaseAuction && a.state.Auction != nil && a.state.Auction.AuctioneerID != playerID {
			legalActions = append(legalActions, "auction.bid")
		}
		if a.state.Phase == PhaseFirstRefusal && a.state.Auction != nil && a.state.Auction.AuctioneerID == playerID {
			legalActions = append(legalActions, "auction.resolve")
		}
		if a.state.Auction != nil && a.state.Auction.HighestBidderID == playerID {
			payment := a.state.Auction.Payment
			bidPayment = &payment
		}
		if a.state.Phase == PhaseTradeResponse && a.state.Trade != nil && a.state.Trade.TargetID == playerID {
			legalActions = append(legalActions, "trade.accept", "trade.counter")
		}
		if a.state.Phase == PhaseTradeReoffer && a.state.Trade != nil && a.state.Trade.ChallengerID == playerID {
			legalActions = append(legalActions, "trade.reoffer")
		}
		if a.state.Phase == PhaseTradeRecounter && a.state.Trade != nil && a.state.Trade.TargetID == playerID {
			legalActions = append(legalActions, "trade.counter")
		}
		if a.state.Trade != nil && a.state.Trade.ChallengerID == playerID {
			offer := a.state.Trade.ChallengerOffer
			ownOffer = &offer
		}
	}
	return Snapshot{
		Version: a.state.Version,
		Public:  public,
		Self:    SelfView{PlayerID: a.state.Players[self].ID, Money: a.state.Players[self].Money, LegalActions: legalActions, BidPayment: bidPayment, OwnOffer: ownOffer},
	}, nil
}

func (a *Aggregate) Public() PublicView {
	public, _ := a.publicView("")
	return public
}

func (a *Aggregate) publicView(playerID string) (PublicView, int) {
	self := -1
	players := make([]PublicPlayer, len(a.state.Players))
	for index, player := range a.state.Players {
		playerAnimals := make(map[Animal]int, len(player.Animals))
		for animal, count := range player.Animals {
			playerAnimals[animal] = count
		}
		players[index] = PublicPlayer{ID: player.ID, Name: player.Name, Seat: index, Animals: playerAnimals, Score: Score(playerAnimals)}
		if player.ID == playerID {
			self = index
		}
	}
	turnPlayerID := ""
	if a.state.Status == StatusPlaying && len(a.state.Players) > 0 {
		turnPlayerID = a.state.Players[a.state.Turn].ID
	}
	var auctionView *AuctionView
	if a.state.Auction != nil {
		auctionView = &AuctionView{
			Animal:          a.state.Auction.Animal,
			AuctioneerID:    a.state.Auction.AuctioneerID,
			HighestBid:      a.state.Auction.HighestBid,
			HighestBidderID: a.state.Auction.HighestBidderID,
		}
	}
	var tradeView *TradeView
	if a.state.Trade != nil {
		tradeView = &TradeView{
			ChallengerID: a.state.Trade.ChallengerID,
			TargetID:     a.state.Trade.TargetID,
			Animal:       a.state.Trade.Animal,
			CardCount:    a.state.Trade.CardCount,
		}
	}
	winnerIDs := []string{}
	if a.state.Status == StatusFinished {
		highest := -1
		for _, player := range players {
			switch {
			case player.Score > highest:
				highest = player.Score
				winnerIDs = []string{player.ID}
			case player.Score == highest:
				winnerIDs = append(winnerIDs, player.ID)
			}
		}
	}
	return PublicView{
		GameID:        a.state.ID,
		Status:        a.state.Status,
		Phase:         a.state.Phase,
		HostID:        a.state.HostID,
		TurnPlayerID:  turnPlayerID,
		Players:       players,
		DeckRemaining: len(a.state.Deck) - a.state.DeckPos,
		Auction:       auctionView,
		Trade:         tradeView,
		WinnerIDs:     winnerIDs,
	}, self
}

func (a *Aggregate) Authenticate(playerID, token string) bool {
	index := a.playerIndex(playerID)
	return index >= 0 && subtle.ConstantTimeCompare([]byte(a.state.Players[index].AuthHash), []byte(HashToken(token))) == 1
}

func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

func (a *Aggregate) requireTurn(actorID string) error {
	if a.state.Status != StatusPlaying || a.state.Phase != PhaseTurn {
		return ruleError("invalid_phase", "game is not waiting for a turn action")
	}
	if a.state.Players[a.state.Turn].ID != actorID {
		return ruleError("not_your_turn", "player is not active")
	}
	return nil
}

func (a *Aggregate) playerIndex(playerID string) int {
	for index := range a.state.Players {
		if a.state.Players[index].ID == playerID {
			return index
		}
	}
	return -1
}

func (a *Aggregate) transferMoney(from, to int, payment Money) error {
	money, err := a.state.Players[from].Money.subtract(payment)
	if err != nil {
		return err
	}
	a.state.Players[from].Money = money
	a.state.Players[to].Money = a.state.Players[to].Money.add(payment)
	return nil
}

func (a *Aggregate) exchangeMoney(first int, firstOffer Money, second int, secondOffer Money) error {
	firstMoney, err := a.state.Players[first].Money.subtract(firstOffer.withoutZero())
	if err != nil {
		return err
	}
	secondMoney, err := a.state.Players[second].Money.subtract(secondOffer.withoutZero())
	if err != nil {
		return err
	}
	a.state.Players[first].Money = firstMoney.add(secondOffer.withoutZero())
	a.state.Players[second].Money = secondMoney.add(firstOffer.withoutZero())
	return nil
}

func (a *Aggregate) transferAnimals(from, to int, animal Animal, count int) error {
	if count <= 0 || a.state.Players[from].Animals[animal] < count {
		return fmt.Errorf("insufficient animals")
	}
	a.state.Players[from].Animals[animal] -= count
	a.state.Players[to].Animals[animal] += count
	return nil
}

func (a *Aggregate) advanceTurn() {
	a.state.Auction = nil
	a.state.Trade = nil
	next := (a.state.Turn + 1) % len(a.state.Players)
	if a.state.DeckPos < len(a.state.Deck) {
		a.state.Turn = next
		a.state.Phase = PhaseTurn
		return
	}
	for offset := range len(a.state.Players) {
		candidate := (next + offset) % len(a.state.Players)
		if a.canTrade(candidate) {
			a.state.Turn = candidate
			a.state.Phase = PhaseTurn
			return
		}
	}
	a.state.Status = StatusFinished
	a.state.Phase = PhaseFinished
}

func (a *Aggregate) canTrade(playerIndex int) bool {
	for animal, count := range a.state.Players[playerIndex].Animals {
		if count == 0 || count == 4 {
			continue
		}
		for other := range a.state.Players {
			if other != playerIndex && a.state.Players[other].Animals[animal] > 0 {
				return true
			}
		}
	}
	return false
}

func validateAuctionPayment(available Money, amount int, payment Money) error {
	if amount <= 0 || !payment.valid() || payment.Zero != 0 || payment.cardCount() == 0 || payment.total() < amount || !available.contains(payment) {
		return ruleError("invalid_payment", "payment cannot cover the bid")
	}
	return nil
}

func donkeyBonus(number int) Money {
	switch number {
	case 1:
		return Money{Fifty: 1}
	case 2:
		return Money{Hundred: 1}
	case 3:
		return Money{TwoHundred: 1}
	case 4:
		return Money{FiveHundred: 1}
	default:
		return Money{}
	}
}

func newEvent(version uint64, eventType string, data any) (Event, error) {
	payload, err := json.Marshal(data)
	if err != nil {
		return Event{}, err
	}
	return Event{Version: version, Type: eventType, Data: payload}, nil
}

func validateIdentity(identity Identity) error {
	if strings.TrimSpace(identity.ID) == "" || strings.TrimSpace(identity.AuthHash) == "" || strings.TrimSpace(identity.Name) == "" {
		return ruleError("invalid_player", "player id, auth hash, and name are required")
	}
	return nil
}

func ruleError(code, message string) *RuleError {
	return &RuleError{Code: code, Message: message}
}
