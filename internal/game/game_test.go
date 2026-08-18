package game_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/khoi/kuhhandel/internal/game"
)

func TestOnlyHostStartsRoomWithThreePlayers(t *testing.T) {
	aggregate := game.New("game-1")
	apply(t, aggregate, "", game.CreateRoom{Player: identity("p1", "Alice")})
	apply(t, aggregate, "", game.JoinRoom{Player: identity("p2", "Bob")})

	_, err := aggregate.Decide("p1", game.StartGame{Seed: 7})
	assertRuleError(t, err, "not_enough_players")

	apply(t, aggregate, "", game.JoinRoom{Player: identity("p3", "Carol")})
	_, err = aggregate.Decide("p2", game.StartGame{Seed: 7})
	assertRuleError(t, err, "forbidden")

	apply(t, aggregate, "p1", game.StartGame{Seed: 7})
	snapshot, err := aggregate.Snapshot("p1")
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Public.HostID != "p1" {
		t.Fatalf("host = %q, want p1", snapshot.Public.HostID)
	}
	if snapshot.Public.Status != game.StatusPlaying {
		t.Fatalf("status = %q, want %q", snapshot.Public.Status, game.StatusPlaying)
	}
	if snapshot.Public.TurnPlayerID != "p1" {
		t.Fatalf("turn = %q, want p1", snapshot.Public.TurnPlayerID)
	}
}

func TestStartDealsPrivateMoneyWithoutLeakingHiddenState(t *testing.T) {
	aggregate := startedGame(t, 19)

	for _, playerID := range []string{"p1", "p2", "p3"} {
		snapshot, err := aggregate.Snapshot(playerID)
		if err != nil {
			t.Fatal(err)
		}
		if snapshot.Self.Money != (game.Money{Zero: 2, Ten: 4, Fifty: 1}) {
			t.Fatalf("%s money = %+v", playerID, snapshot.Self.Money)
		}
		if snapshot.Public.DeckRemaining != 40 {
			t.Fatalf("deck remaining = %d, want 40", snapshot.Public.DeckRemaining)
		}
		encoded, err := json.Marshal(snapshot.Public)
		if err != nil {
			t.Fatal(err)
		}
		publicJSON := string(encoded)
		for _, secret := range []string{"token", "money", "seed", playerID + "-token"} {
			if strings.Contains(publicJSON, secret) {
				t.Fatalf("public state leaked %q: %s", secret, publicJSON)
			}
		}
	}

	host, err := aggregate.Snapshot("p1")
	if err != nil {
		t.Fatal(err)
	}
	if len(host.Self.LegalActions) != 1 || host.Self.LegalActions[0] != "turn.auction" {
		t.Fatalf("host actions = %v", host.Self.LegalActions)
	}
	other, err := aggregate.Snapshot("p2")
	if err != nil {
		t.Fatal(err)
	}
	if len(other.Self.LegalActions) != 0 {
		t.Fatalf("non-active player actions = %v", other.Self.LegalActions)
	}
}

func TestAuctionWithoutBidAwardsAnimalAndAdvancesTurn(t *testing.T) {
	aggregate := startedGame(t, 31)
	apply(t, aggregate, "p1", game.BeginAuction{})

	revealed, err := aggregate.Snapshot("p2")
	if err != nil {
		t.Fatal(err)
	}
	if revealed.Public.Phase != game.PhaseAuction {
		t.Fatalf("phase = %q, want %q", revealed.Public.Phase, game.PhaseAuction)
	}
	if revealed.Public.Auction == nil || revealed.Public.Auction.Animal == "" {
		t.Fatalf("auction = %+v", revealed.Public.Auction)
	}
	if revealed.Public.DeckRemaining != 39 {
		t.Fatalf("deck remaining = %d, want 39", revealed.Public.DeckRemaining)
	}
	animal := revealed.Public.Auction.Animal

	apply(t, aggregate, "p1", game.CloseAuction{})
	settled, err := aggregate.Snapshot("p3")
	if err != nil {
		t.Fatal(err)
	}
	if settled.Public.Phase != game.PhaseTurn {
		t.Fatalf("phase = %q, want %q", settled.Public.Phase, game.PhaseTurn)
	}
	if settled.Public.TurnPlayerID != "p2" {
		t.Fatalf("turn = %q, want p2", settled.Public.TurnPlayerID)
	}
	if settled.Public.Players[0].Animals[animal] != 1 {
		t.Fatalf("host animals = %v, want one %s", settled.Public.Players[0].Animals, animal)
	}
}

func TestEachDonkeyPaysEveryPlayerBeforeAuction(t *testing.T) {
	aggregate := startedGame(t, 41)
	for range 40 {
		active := mustSnapshot(t, aggregate, "p1").Public.TurnPlayerID
		apply(t, aggregate, active, game.BeginAuction{})
		apply(t, aggregate, active, game.CloseAuction{})
	}

	want := game.Money{Zero: 2, Ten: 4, Fifty: 2, Hundred: 1, TwoHundred: 1, FiveHundred: 1}
	for _, playerID := range []string{"p1", "p2", "p3"} {
		got := mustSnapshot(t, aggregate, playerID).Self.Money
		if got != want {
			t.Fatalf("%s money = %+v, want %+v", playerID, got, want)
		}
	}
}

func TestHighestBidderPaysAuctioneerAndTakesAnimal(t *testing.T) {
	aggregate := startedGame(t, 1)
	apply(t, aggregate, "p1", game.BeginAuction{})
	animal := mustSnapshot(t, aggregate, "p1").Public.Auction.Animal
	apply(t, aggregate, "p2", game.PlaceBid{Amount: 20, Payment: game.Money{Ten: 2}})

	public := mustSnapshot(t, aggregate, "p3").Public
	if public.Auction.HighestBid != 20 || public.Auction.HighestBidderID != "p2" {
		t.Fatalf("highest bid = %+v", public.Auction)
	}
	_, err := aggregate.Decide("p3", game.PlaceBid{Amount: 20, Payment: game.Money{Ten: 2}})
	assertRuleError(t, err, "bid_too_low")

	apply(t, aggregate, "p1", game.CloseAuction{})
	if phase := mustSnapshot(t, aggregate, "p1").Public.Phase; phase != game.PhaseFirstRefusal {
		t.Fatalf("phase = %q, want %q", phase, game.PhaseFirstRefusal)
	}
	apply(t, aggregate, "p1", game.ResolveAuction{Buy: false})

	settled := mustSnapshot(t, aggregate, "p1")
	if settled.Public.Players[1].Animals[animal] != 1 {
		t.Fatalf("bidder animals = %v", settled.Public.Players[1].Animals)
	}
	if settled.Self.Money != (game.Money{Zero: 2, Ten: 6, Fifty: 1}) {
		t.Fatalf("auctioneer money = %+v", settled.Self.Money)
	}
	if bidder := mustSnapshot(t, aggregate, "p2").Self.Money; bidder != (game.Money{Zero: 2, Ten: 2, Fifty: 1}) {
		t.Fatalf("bidder money = %+v", bidder)
	}
}

func TestAuctioneerPaysChosenCardsWhenUsingFirstRefusal(t *testing.T) {
	aggregate := startedGame(t, 5)
	apply(t, aggregate, "p1", game.BeginAuction{})
	animal := mustSnapshot(t, aggregate, "p1").Public.Auction.Animal
	apply(t, aggregate, "p2", game.PlaceBid{Amount: 20, Payment: game.Money{Ten: 2}})
	apply(t, aggregate, "p1", game.CloseAuction{})
	beforeHost := mustSnapshot(t, aggregate, "p1").Self.Money
	beforeBidder := mustSnapshot(t, aggregate, "p2").Self.Money
	apply(t, aggregate, "p1", game.ResolveAuction{Buy: true, Payment: game.Money{Fifty: 1}})

	afterHost := mustSnapshot(t, aggregate, "p1")
	afterBidder := mustSnapshot(t, aggregate, "p2")
	if publicPlayer(afterHost.Public, "p1").Animals[animal] != 1 {
		t.Fatalf("auctioneer animals = %+v", publicPlayer(afterHost.Public, "p1").Animals)
	}
	if afterHost.Self.Money.Fifty != beforeHost.Fifty-1 || afterBidder.Self.Money.Fifty != beforeBidder.Fifty+1 {
		t.Fatalf("money transfer host=%+v bidder=%+v", afterHost.Self.Money, afterBidder.Self.Money)
	}
	if afterBidder.Self.Money.Ten != beforeBidder.Ten {
		t.Fatalf("bid payment was charged: before=%+v after=%+v", beforeBidder, afterBidder.Self.Money)
	}
}

func TestRoomRejectsSixthPlayerAndDuplicateIdentity(t *testing.T) {
	aggregate := game.New("game-1")
	apply(t, aggregate, "", game.CreateRoom{Player: identity("p1", "Alice")})
	for index := 2; index <= 5; index++ {
		playerID := fmt.Sprintf("p%d", index)
		apply(t, aggregate, "", game.JoinRoom{Player: identity(playerID, playerID)})
	}
	_, err := aggregate.Decide("", game.JoinRoom{Player: identity("p6", "Six")})
	assertRuleError(t, err, "room_full")

	other := game.New("game-2")
	apply(t, other, "", game.CreateRoom{Player: identity("p1", "Alice")})
	_, err = other.Decide("", game.JoinRoom{Player: identity("p1", "Again")})
	assertRuleError(t, err, "player_exists")
	_, err = other.Decide("", game.JoinRoom{Player: game.Identity{ID: "new", Token: "p1-token", Name: "Again"}})
	assertRuleError(t, err, "player_exists")
}

func TestAuctionRejectsInvalidActorsAndPayments(t *testing.T) {
	aggregate := startedGame(t, 11)
	_, err := aggregate.Decide("p2", game.BeginAuction{})
	assertRuleError(t, err, "not_your_turn")
	apply(t, aggregate, "p1", game.BeginAuction{})

	cases := []struct {
		name    string
		actorID string
		bid     game.PlaceBid
		code    string
	}{
		{name: "auctioneer", actorID: "p1", bid: game.PlaceBid{Amount: 10, Payment: game.Money{Ten: 1}}, code: "forbidden"},
		{name: "unknown", actorID: "outsider", bid: game.PlaceBid{Amount: 10, Payment: game.Money{Ten: 1}}, code: "not_a_player"},
		{name: "underpay", actorID: "p2", bid: game.PlaceBid{Amount: 20, Payment: game.Money{Ten: 1}}, code: "invalid_payment"},
		{name: "zero", actorID: "p2", bid: game.PlaceBid{Amount: 10, Payment: game.Money{Zero: 1}}, code: "invalid_payment"},
		{name: "negative", actorID: "p2", bid: game.PlaceBid{Amount: 10, Payment: game.Money{Ten: -1, Fifty: 1}}, code: "invalid_payment"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			_, err := aggregate.Decide(test.actorID, test.bid)
			assertRuleError(t, err, test.code)
		})
	}
	apply(t, aggregate, "p2", game.PlaceBid{Amount: 20, Payment: game.Money{Fifty: 1}})
	apply(t, aggregate, "p1", game.CloseAuction{})
	_, err = aggregate.Decide("p2", game.ResolveAuction{Buy: false})
	assertRuleError(t, err, "forbidden")
	_, err = aggregate.Decide("p1", game.ResolveAuction{Buy: false, Payment: game.Money{Ten: 1}})
	assertRuleError(t, err, "invalid_payment")
	_, err = aggregate.Decide("p1", game.ResolveAuction{Buy: true, Payment: game.Money{Ten: 1}})
	assertRuleError(t, err, "invalid_payment")
}

func TestAcceptedTradeTransfersAnimalAndPositiveMoneyWithoutLeakingOffer(t *testing.T) {
	aggregate := startedGame(t, 67)
	challengerID, targetID, animal := reachTradeOpportunity(t, aggregate)
	beforeChallenger := mustSnapshot(t, aggregate, challengerID)
	beforeTarget := mustSnapshot(t, aggregate, targetID)
	offer := game.Money{Zero: 1, Ten: 1}
	apply(t, aggregate, challengerID, game.BeginTrade{TargetID: targetID, Animal: animal, Offer: offer})

	pending := mustSnapshot(t, aggregate, targetID)
	if pending.Public.Phase != game.PhaseTradeResponse || pending.Public.Trade == nil {
		t.Fatalf("pending trade = %+v", pending.Public)
	}
	encoded, err := json.Marshal(pending.Public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "offer") || strings.Contains(string(encoded), "money") {
		t.Fatalf("public trade leaked hidden offer: %s", encoded)
	}
	if pending.Self.OwnOffer != nil {
		t.Fatalf("target can see challenger offer: %+v", pending.Self.OwnOffer)
	}
	count := pending.Public.Trade.CardCount
	apply(t, aggregate, targetID, game.AcceptTrade{})

	afterChallenger := mustSnapshot(t, aggregate, challengerID)
	afterTarget := mustSnapshot(t, aggregate, targetID)
	if got := publicPlayer(afterChallenger.Public, challengerID).Animals[animal]; got != publicPlayer(beforeChallenger.Public, challengerID).Animals[animal]+count {
		t.Fatalf("challenger %s count = %d", animal, got)
	}
	if got := publicPlayer(afterTarget.Public, targetID).Animals[animal]; got != publicPlayer(beforeTarget.Public, targetID).Animals[animal]-count {
		t.Fatalf("target %s count = %d", animal, got)
	}
	if afterChallenger.Self.Money.Zero != beforeChallenger.Self.Money.Zero || afterChallenger.Self.Money.Ten != beforeChallenger.Self.Money.Ten-1 {
		t.Fatalf("challenger money = %+v, before %+v", afterChallenger.Self.Money, beforeChallenger.Self.Money)
	}
	if afterTarget.Self.Money.Zero != beforeTarget.Self.Money.Zero || afterTarget.Self.Money.Ten != beforeTarget.Self.Money.Ten+1 {
		t.Fatalf("target money = %+v, before %+v", afterTarget.Self.Money, beforeTarget.Self.Money)
	}
}

func TestHigherCounterofferWinsAnimalAndExchangesBids(t *testing.T) {
	aggregate := startedGame(t, 79)
	challengerID, targetID, animal := reachTradeOpportunity(t, aggregate)
	beforeChallenger := mustSnapshot(t, aggregate, challengerID)
	beforeTarget := mustSnapshot(t, aggregate, targetID)
	apply(t, aggregate, challengerID, game.BeginTrade{TargetID: targetID, Animal: animal, Offer: game.Money{Ten: 1}})
	count := mustSnapshot(t, aggregate, targetID).Public.Trade.CardCount
	apply(t, aggregate, targetID, game.CounterTrade{Offer: game.Money{Fifty: 1}})

	afterChallenger := mustSnapshot(t, aggregate, challengerID)
	afterTarget := mustSnapshot(t, aggregate, targetID)
	if got := publicPlayer(afterTarget.Public, targetID).Animals[animal]; got != publicPlayer(beforeTarget.Public, targetID).Animals[animal]+count {
		t.Fatalf("target %s count = %d", animal, got)
	}
	if got := publicPlayer(afterChallenger.Public, challengerID).Animals[animal]; got != publicPlayer(beforeChallenger.Public, challengerID).Animals[animal]-count {
		t.Fatalf("challenger %s count = %d", animal, got)
	}
	if afterChallenger.Self.Money.Ten != beforeChallenger.Self.Money.Ten-1 || afterChallenger.Self.Money.Fifty != beforeChallenger.Self.Money.Fifty+1 {
		t.Fatalf("challenger money = %+v, before %+v", afterChallenger.Self.Money, beforeChallenger.Self.Money)
	}
	if afterTarget.Self.Money.Ten != beforeTarget.Self.Money.Ten+1 || afterTarget.Self.Money.Fifty != beforeTarget.Self.Money.Fifty-1 {
		t.Fatalf("target money = %+v, before %+v", afterTarget.Self.Money, beforeTarget.Self.Money)
	}
}

func TestSecondTiedTradeGivesAnimalToChallengerAndReturnsZeroCards(t *testing.T) {
	aggregate := startedGame(t, 83)
	challengerID, targetID, animal := reachTradeOpportunity(t, aggregate)
	beforeChallenger := mustSnapshot(t, aggregate, challengerID)
	beforeTarget := mustSnapshot(t, aggregate, targetID)
	apply(t, aggregate, challengerID, game.BeginTrade{TargetID: targetID, Animal: animal, Offer: game.Money{Ten: 1}})
	count := mustSnapshot(t, aggregate, targetID).Public.Trade.CardCount
	apply(t, aggregate, targetID, game.CounterTrade{Offer: game.Money{Ten: 1}})
	if phase := mustSnapshot(t, aggregate, challengerID).Public.Phase; phase != game.PhaseTradeReoffer {
		t.Fatalf("phase = %q, want %q", phase, game.PhaseTradeReoffer)
	}
	apply(t, aggregate, challengerID, game.ReofferTrade{Offer: game.Money{Zero: 1}})
	apply(t, aggregate, targetID, game.CounterTrade{Offer: game.Money{Zero: 1}})

	afterChallenger := mustSnapshot(t, aggregate, challengerID)
	afterTarget := mustSnapshot(t, aggregate, targetID)
	if got := publicPlayer(afterChallenger.Public, challengerID).Animals[animal]; got != publicPlayer(beforeChallenger.Public, challengerID).Animals[animal]+count {
		t.Fatalf("challenger %s count = %d", animal, got)
	}
	if afterChallenger.Self.Money.Zero != beforeChallenger.Self.Money.Zero || afterTarget.Self.Money.Zero != beforeTarget.Self.Money.Zero {
		t.Fatalf("zero cards changed: challenger %d→%d target %d→%d", beforeChallenger.Self.Money.Zero, afterChallenger.Self.Money.Zero, beforeTarget.Self.Money.Zero, afterTarget.Self.Money.Zero)
	}
}

func TestScoreMatchesRulebookExample(t *testing.T) {
	sets := map[game.Animal]int{
		game.AnimalPig:     4,
		game.AnimalDog:     4,
		game.AnimalRooster: 4,
		game.AnimalHorse:   3,
	}
	if score := game.Score(sets); score != 2460 {
		t.Fatalf("score = %d, want 2460", score)
	}
}

func TestCompleteGamesConserveCardsAndEndWithOnlyCompleteSets(t *testing.T) {
	for _, playerCount := range []int{3, 4, 5} {
		for _, seed := range []uint64{0, 1, 97, ^uint64(0)} {
			t.Run(fmt.Sprintf("%d-players-seed-%d", playerCount, seed), func(t *testing.T) {
				completeGame(t, startedGamePlayers(t, seed, playerCount))
			})
		}
	}
}

func TestEventsReplayToIdenticalPublicAndPrivateState(t *testing.T) {
	aggregate := game.New("game-1")
	history := []game.Event{}
	commands := []struct {
		actorID string
		command game.Command
	}{
		{command: game.CreateRoom{Player: identity("p1", "Alice")}},
		{command: game.JoinRoom{Player: identity("p2", "Bob")}},
		{command: game.JoinRoom{Player: identity("p3", "Carol")}},
		{actorID: "p1", command: game.StartGame{Seed: 101}},
		{actorID: "p1", command: game.BeginAuction{}},
		{actorID: "p2", command: game.PlaceBid{Amount: 10, Payment: game.Money{Ten: 1}}},
		{actorID: "p1", command: game.CloseAuction{}},
		{actorID: "p1", command: game.ResolveAuction{Buy: false}},
	}
	for _, item := range commands {
		event, err := aggregate.Decide(item.actorID, item.command)
		if err != nil {
			t.Fatal(err)
		}
		if err := aggregate.Apply(event); err != nil {
			t.Fatal(err)
		}
		history = append(history, event)
	}

	replayed := game.New("game-1")
	for _, event := range history {
		if err := replayed.Apply(event); err != nil {
			t.Fatal(err)
		}
	}
	for _, playerID := range []string{"p1", "p2", "p3"} {
		if got, want := mustSnapshot(t, replayed, playerID), mustSnapshot(t, aggregate, playerID); !reflect.DeepEqual(got, want) {
			t.Fatalf("%s replay = %+v, want %+v", playerID, got, want)
		}
	}
}

func completeGame(t *testing.T, aggregate *game.Aggregate) {
	t.Helper()
	for range 40 {
		active := mustSnapshot(t, aggregate, "p1").Public.TurnPlayerID
		apply(t, aggregate, active, game.BeginAuction{})
		apply(t, aggregate, active, game.CloseAuction{})
	}

	for range 100 {
		public := mustSnapshot(t, aggregate, "p1").Public
		if public.Status == game.StatusFinished {
			break
		}
		challenger := publicPlayer(public, public.TurnPlayerID)
		targetID, animal := tradeTarget(challenger, public.Players)
		if targetID == "" {
			t.Fatalf("active player %s has no trade while game continues", challenger.ID)
		}
		apply(t, aggregate, challenger.ID, game.BeginTrade{TargetID: targetID, Animal: animal, Offer: game.Money{Zero: 1}})
		apply(t, aggregate, targetID, game.AcceptTrade{})
	}

	public := mustSnapshot(t, aggregate, "p1").Public
	if public.Status != game.StatusFinished || public.Phase != game.PhaseFinished {
		t.Fatalf("game did not finish: status=%q phase=%q", public.Status, public.Phase)
	}
	totals := map[game.Animal]int{}
	for _, player := range public.Players {
		for animal, count := range player.Animals {
			totals[animal] += count
			if count != 0 && count != 4 {
				t.Fatalf("%s owns incomplete %s set of %d", player.ID, animal, count)
			}
		}
	}
	if len(totals) != 10 {
		t.Fatalf("animal types = %d, want 10", len(totals))
	}
	for animal, count := range totals {
		if count != 4 {
			t.Fatalf("total %s cards = %d, want 4", animal, count)
		}
	}
	if len(public.WinnerIDs) == 0 {
		t.Fatal("finished game has no winner")
	}
}

func identity(id, name string) game.Identity {
	return game.Identity{ID: id, Token: id + "-token", Name: name}
}

func startedGame(t *testing.T, seed uint64) *game.Aggregate {
	t.Helper()
	return startedGamePlayers(t, seed, 3)
}

func startedGamePlayers(t *testing.T, seed uint64, playerCount int) *game.Aggregate {
	t.Helper()
	aggregate := game.New("game-1")
	apply(t, aggregate, "", game.CreateRoom{Player: identity("p1", "Alice")})
	for index := 2; index <= playerCount; index++ {
		playerID := fmt.Sprintf("p%d", index)
		apply(t, aggregate, "", game.JoinRoom{Player: identity(playerID, playerID)})
	}
	apply(t, aggregate, "p1", game.StartGame{Seed: seed})
	return aggregate
}

func mustSnapshot(t *testing.T, aggregate *game.Aggregate, playerID string) game.Snapshot {
	t.Helper()
	snapshot, err := aggregate.Snapshot(playerID)
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

func reachTradeOpportunity(t *testing.T, aggregate *game.Aggregate) (string, string, game.Animal) {
	t.Helper()
	for range 40 {
		public := mustSnapshot(t, aggregate, "p1").Public
		challengerID := public.TurnPlayerID
		challenger := publicPlayer(public, challengerID)
		for animal, count := range challenger.Animals {
			if count == 0 || count == 4 {
				continue
			}
			for _, target := range public.Players {
				if target.ID != challengerID && target.Animals[animal] > 0 {
					return challengerID, target.ID, animal
				}
			}
		}
		apply(t, aggregate, challengerID, game.BeginAuction{})
		apply(t, aggregate, challengerID, game.CloseAuction{})
	}
	t.Fatal("no trade opportunity reached")
	return "", "", ""
}

func publicPlayer(public game.PublicView, playerID string) game.PublicPlayer {
	for _, player := range public.Players {
		if player.ID == playerID {
			return player
		}
	}
	return game.PublicPlayer{}
}

func tradeTarget(challenger game.PublicPlayer, players []game.PublicPlayer) (string, game.Animal) {
	for animal, count := range challenger.Animals {
		if count == 0 || count == 4 {
			continue
		}
		for _, target := range players {
			if target.ID != challenger.ID && target.Animals[animal] > 0 {
				return target.ID, animal
			}
		}
	}
	return "", ""
}

func apply(t *testing.T, aggregate *game.Aggregate, actorID string, command game.Command) {
	t.Helper()
	event, err := aggregate.Decide(actorID, command)
	if err != nil {
		t.Fatal(err)
	}
	if err := aggregate.Apply(event); err != nil {
		t.Fatal(err)
	}
}

func assertRuleError(t *testing.T, err error, code string) {
	t.Helper()
	var ruleError *game.RuleError
	if !errors.As(err, &ruleError) {
		t.Fatalf("error = %v, want rule error %q", err, code)
	}
	if ruleError.Code != code {
		t.Fatalf("error code = %q, want %q", ruleError.Code, code)
	}
}
