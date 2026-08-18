package server_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/khoi/kuhhandel/internal/game"
	"github.com/khoi/kuhhandel/internal/server"
)

func TestWebSocketHostCreatesAndStartsThreePlayerGame(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, url)
	if room.created.Type != "snapshot" || room.hostSession.GameID == "" {
		t.Fatalf("create response = %+v", room.created)
	}

	rejected := room.bob.request(t, "bad-start", "game.start", nil)
	if rejected.Type != "error" || rejected.Error == nil || rejected.Error.Code != "forbidden" {
		t.Fatalf("non-host start = %+v", rejected)
	}
	started := room.start(t)
	if started.Game.Public.Status != game.StatusPlaying || len(started.Game.Public.Players) != 3 {
		t.Fatalf("started game = %+v", started.Game.Public)
	}
	if started.Game.Self.Money != (game.Money{Zero: 2, Ten: 4, Fifty: 1}) {
		t.Fatalf("host money = %+v", started.Game.Self.Money)
	}
	publicJSON, err := json.Marshal(started.Game.Public)
	if err != nil {
		t.Fatal(err)
	}
	for _, hidden := range []string{"money", "token", "seed", room.hostSession.Token} {
		if strings.Contains(string(publicJSON), hidden) {
			t.Fatalf("public state leaked %q: %s", hidden, publicJSON)
		}
	}
	bobView := room.bob.awaitVersion(t, started.Game.Version)
	encodedBobView, err := json.Marshal(bobView)
	if err != nil {
		t.Fatal(err)
	}
	if bobView.Session != nil || strings.Contains(string(encodedBobView), room.hostSession.Token) {
		t.Fatalf("broadcast leaked host session: %s", encodedBobView)
	}
}

func TestWebSocketSessionResumesAfterServerRestart(t *testing.T) {
	databasePath := t.TempDir() + "/games.db"
	application, httpServer, url := newTestServer(t, databasePath)
	room := newRoom(t, url)
	started := room.start(t)
	httpServer.Close()
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}

	_, _, restartedURL := newTestServer(t, databasePath)

	intruder := newClient(t, restartedURL)
	rejected := intruder.request(t, "bad-resume", "session.resume", map[string]any{
		"gameId": room.hostSession.GameID, "playerId": room.hostSession.PlayerID, "token": "wrong",
	})
	if rejected.Type != "error" || rejected.Error == nil || rejected.Error.Code != "unauthorized" {
		t.Fatalf("bad resume = %+v", rejected)
	}

	resumedClient := newClient(t, restartedURL)
	resumed := resumedClient.request(t, "resume", "session.resume", room.hostSession)
	if resumed.Type != "snapshot" || resumed.Session == nil || *resumed.Session != room.hostSession {
		t.Fatalf("resume response = %+v", resumed)
	}
	if resumed.Game.Version != started.Game.Version || resumed.Game.Public.Status != game.StatusPlaying {
		t.Fatalf("resumed game = %+v, started game = %+v", resumed.Game, started.Game)
	}
	if !reflect.DeepEqual(resumed.Game.Self, started.Game.Self) {
		t.Fatalf("resumed self = %+v, started self = %+v", resumed.Game.Self, started.Game.Self)
	}
}

func TestWebSocketAuctionKeepsPaymentPrivateAndSettlesOnServer(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, url)
	room.start(t)

	auction := room.host.request(t, "auction", "turn.auction", nil)
	if auction.Type != "snapshot" || auction.Game.Public.Auction == nil {
		t.Fatalf("auction response = %+v", auction)
	}
	animal := auction.Game.Public.Auction.Animal
	bid := room.bob.request(t, "bid", "auction.bid", map[string]any{"amount": 10, "payment": game.Money{Ten: 1}})
	if bid.Game.Public.Auction.HighestBid != 10 || bid.Game.Public.Auction.HighestBidderID != room.bobSession.PlayerID {
		t.Fatalf("bid response = %+v", bid.Game.Public.Auction)
	}
	if bid.Game.Self.BidPayment == nil || *bid.Game.Self.BidPayment != (game.Money{Ten: 1}) {
		t.Fatalf("bidder payment view = %+v", bid.Game.Self.BidPayment)
	}
	if observer := room.carol.awaitVersion(t, bid.Game.Version); observer.Game.Self.BidPayment != nil {
		t.Fatalf("observer saw bidder payment: %+v", observer.Game.Self.BidPayment)
	}
	publicJSON, err := json.Marshal(bid.Game.Public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "payment") {
		t.Fatalf("public auction leaked payment: %s", publicJSON)
	}
	closed := room.host.request(t, "close", "auction.close", nil)
	if closed.Game.Public.Phase != game.PhaseFirstRefusal {
		t.Fatalf("closed auction = %+v", closed.Game.Public)
	}
	settled := room.host.request(t, "sell", "auction.resolve", map[string]any{"buy": false})
	if settled.Game.Public.Phase != game.PhaseTurn || settled.Game.Public.Auction != nil {
		t.Fatalf("settled auction = %+v", settled.Game.Public)
	}
	wantHostMoney := closed.Game.Self.Money
	wantHostMoney.Ten++
	if settled.Game.Self.Money != wantHostMoney {
		t.Fatalf("host money = %+v", settled.Game.Self.Money)
	}
	bobSnapshot := room.bob.awaitVersion(t, settled.Game.Version)
	wantBobMoney := bid.Game.Self.Money
	wantBobMoney.Ten--
	if bobSnapshot.Game.Self.Money != wantBobMoney {
		t.Fatalf("bidder money = %+v", bobSnapshot.Game.Self.Money)
	}
	if animalCount(bobSnapshot.Game.Public, room.bobSession.PlayerID, animal) != 1 {
		t.Fatalf("bidder animals = %+v", bobSnapshot.Game.Public.Players)
	}
}

func TestWebSocketTradeRevealsOfferOnlyToOwnerAndSettlesOnServer(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, url)
	view, challengerID, targetID, animal := room.reachTrade(t, "accept")
	originalTargetCount := animalCount(view, targetID, animal)
	originalChallengerCount := animalCount(view, challengerID, animal)
	trade := room.clients[challengerID].request(t, "trade", "turn.trade", map[string]any{
		"targetId": targetID, "animal": animal, "offer": game.Money{Zero: 1},
	})
	if trade.Type != "snapshot" || trade.Game.Self.OwnOffer == nil || *trade.Game.Self.OwnOffer != (game.Money{Zero: 1}) {
		t.Fatalf("challenger trade view = %+v", trade)
	}
	publicJSON, err := json.Marshal(trade.Game.Public)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "offer") {
		t.Fatalf("public trade leaked offer: %s", publicJSON)
	}
	targetView := room.clients[targetID].awaitVersion(t, trade.Game.Version)
	if targetView.Game.Self.OwnOffer != nil {
		t.Fatalf("target saw hidden offer: %+v", targetView.Game.Self.OwnOffer)
	}
	accepted := room.clients[targetID].request(t, "accept", "trade.accept", nil)
	if accepted.Type != "snapshot" || accepted.Game.Public.Trade != nil {
		t.Fatalf("accepted trade = %+v", accepted)
	}
	cardCount := trade.Game.Public.Trade.CardCount
	if animalCount(accepted.Game.Public, targetID, animal) != originalTargetCount-cardCount || animalCount(accepted.Game.Public, challengerID, animal) != originalChallengerCount+cardCount {
		t.Fatalf("settled animals = %+v", accepted.Game.Public.Players)
	}
}

func TestWebSocketSecondTiedTradeAwardsAnimalToChallenger(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, url)
	view, challengerID, targetID, animal := room.reachTrade(t, "tie")
	beforeChallenger := animalCount(view, challengerID, animal)
	beforeTarget := animalCount(view, targetID, animal)
	startedTrade := room.clients[challengerID].request(t, "tie-trade", "turn.trade", map[string]any{
		"targetId": targetID, "animal": animal, "offer": game.Money{Zero: 1},
	})
	cardCount := startedTrade.Game.Public.Trade.CardCount
	firstTie := room.clients[targetID].request(t, "first-tie", "trade.counter", map[string]any{"offer": game.Money{Zero: 1}})
	if firstTie.Game.Public.Phase != game.PhaseTradeReoffer {
		t.Fatalf("first tie = %+v", firstTie)
	}
	reoffer := room.clients[challengerID].request(t, "reoffer", "trade.reoffer", map[string]any{"offer": game.Money{Zero: 1}})
	if reoffer.Game.Public.Phase != game.PhaseTradeRecounter {
		t.Fatalf("reoffer = %+v", reoffer)
	}
	settled := room.clients[targetID].request(t, "second-tie", "trade.counter", map[string]any{"offer": game.Money{Zero: 1}})
	if settled.Type != "snapshot" || settled.Game.Public.Trade != nil {
		t.Fatalf("second tie = %+v", settled)
	}
	if animalCount(settled.Game.Public, challengerID, animal) != beforeChallenger+cardCount || animalCount(settled.Game.Public, targetID, animal) != beforeTarget-cardCount {
		t.Fatalf("settled animals = %+v", settled.Game.Public.Players)
	}
}

func TestWebSocketSerializesSimultaneousBids(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, url)
	room.start(t)
	room.host.request(t, "auction", "turn.auction", nil)

	type result struct {
		response response
		err      error
	}
	results := make(chan result, 2)
	for index, client := range []*testClient{room.bob, room.carol} {
		go func(index int, client *testClient) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			response, err := client.do(ctx, fmt.Sprintf("bid-%d", index), "auction.bid", map[string]any{"amount": 10, "payment": game.Money{Ten: 1}})
			results <- result{response: response, err: err}
		}(index, client)
	}
	accepted := 0
	rejected := 0
	for range 2 {
		result := <-results
		if result.err != nil {
			t.Fatal(result.err)
		}
		switch {
		case result.response.Type == "snapshot":
			accepted++
			if result.response.Game.Public.Auction.HighestBid != 10 {
				t.Fatalf("accepted bid = %+v", result.response)
			}
		case result.response.Type == "error" && result.response.Error != nil && result.response.Error.Code == "bid_too_low":
			rejected++
		default:
			t.Fatalf("bid result = %+v", result.response)
		}
	}
	if accepted != 1 || rejected != 1 {
		t.Fatalf("accepted=%d rejected=%d", accepted, rejected)
	}
}

func TestWebSocketCompleteGamePersistsFinalResult(t *testing.T) {
	databasePath := t.TempDir() + "/games.db"
	application, httpServer, url := newTestServer(t, databasePath)
	room := newRoom(t, url)
	view := room.start(t).Game.Public
	for turn := 0; turn < 40; turn++ {
		active := view.TurnPlayerID
		room.syncedRequest(t, active, fmt.Sprintf("full-auction-%d", turn), "turn.auction", nil)
		view = room.syncedRequest(t, active, fmt.Sprintf("full-close-%d", turn), "auction.close", nil).Game.Public
	}
	for trade := 0; trade < 100 && view.Status != game.StatusFinished; trade++ {
		challengerID, targetID, animal := findTrade(view)
		if challengerID == "" {
			t.Fatalf("turn player has no trade: %+v", view)
		}
		room.syncedRequest(t, challengerID, fmt.Sprintf("full-trade-%d", trade), "turn.trade", map[string]any{
			"targetId": targetID, "animal": animal, "offer": game.Money{Zero: 1},
		})
		view = room.syncedRequest(t, targetID, fmt.Sprintf("full-accept-%d", trade), "trade.accept", nil).Game.Public
	}
	if view.Status != game.StatusFinished || len(view.WinnerIDs) == 0 {
		t.Fatalf("final game = %+v", view)
	}
	for _, player := range view.Players {
		for animal, count := range player.Animals {
			if count != 0 && count != 4 {
				t.Fatalf("%s owns %d %s cards", player.ID, count, animal)
			}
		}
	}
	httpServer.Close()
	if err := application.Close(); err != nil {
		t.Fatal(err)
	}
	_, _, restartedURL := newTestServer(t, databasePath)
	resumed := newClient(t, restartedURL).request(t, "resume-final", "session.resume", room.hostSession)
	if !reflect.DeepEqual(resumed.Game.Public, view) {
		t.Fatalf("replayed final game = %+v, want %+v", resumed.Game.Public, view)
	}
}

func TestWebSocketKeepsConcurrentGamesIsolated(t *testing.T) {
	_, _, url := newTestServer(t, t.TempDir()+"/games.db")
	first := newRoom(t, url)
	second := newRoom(t, url)
	if first.hostSession.GameID == second.hostSession.GameID {
		t.Fatalf("duplicate game id %q", first.hostSession.GameID)
	}
	started := first.start(t)
	if started.Game.Public.Status != game.StatusPlaying {
		t.Fatalf("first game = %+v", started.Game.Public)
	}
	rejected := second.host.request(t, "early-auction", "turn.auction", nil)
	if rejected.Error == nil || rejected.Error.Code != "invalid_phase" {
		t.Fatalf("second game changed with first: %+v", rejected)
	}
	secondStarted := second.start(t)
	if secondStarted.Game.Public.Status != game.StatusPlaying || secondStarted.Game.Public.GameID != second.hostSession.GameID {
		t.Fatalf("second game = %+v", secondStarted.Game.Public)
	}
}

type testRoom struct {
	host         *testClient
	bob          *testClient
	carol        *testClient
	hostSession  session
	bobSession   session
	carolSession session
	clients      map[string]*testClient
	created      response
}

func newTestServer(t *testing.T, databasePath string) (*server.Server, *httptest.Server, string) {
	t.Helper()
	application, err := server.New(databasePath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { application.Close() })
	httpServer := httptest.NewServer(application.Handler())
	t.Cleanup(httpServer.Close)
	url := "ws" + strings.TrimPrefix(httpServer.URL, "http") + "/ws"
	return application, httpServer, url
}

func newRoom(t *testing.T, url string) testRoom {
	t.Helper()
	host := newClient(t, url)
	bob := newClient(t, url)
	carol := newClient(t, url)
	created := host.request(t, "create", "room.create", map[string]any{"name": "Alice"})
	if created.Session == nil {
		t.Fatalf("create response = %+v", created)
	}
	hostSession := *created.Session
	bobJoined := bob.request(t, "join-bob", "room.join", map[string]any{"gameId": hostSession.GameID, "name": "Bob"})
	carolJoined := carol.request(t, "join-carol", "room.join", map[string]any{"gameId": hostSession.GameID, "name": "Carol"})
	if bobJoined.Session == nil || carolJoined.Session == nil {
		t.Fatalf("join responses: bob=%+v carol=%+v", bobJoined, carolJoined)
	}
	room := testRoom{
		host:         host,
		bob:          bob,
		carol:        carol,
		hostSession:  hostSession,
		bobSession:   *bobJoined.Session,
		carolSession: *carolJoined.Session,
		created:      created,
	}
	room.clients = map[string]*testClient{
		room.hostSession.PlayerID:  host,
		room.bobSession.PlayerID:   bob,
		room.carolSession.PlayerID: carol,
	}
	return room
}

func (room testRoom) start(t *testing.T) response {
	t.Helper()
	return room.host.request(t, "start", "game.start", nil)
}

func (room testRoom) reachTrade(t *testing.T, prefix string) (game.PublicView, string, string, game.Animal) {
	t.Helper()
	view := room.start(t).Game.Public
	for turn := 0; turn < 40; turn++ {
		challengerID, targetID, animal := findTrade(view)
		if challengerID != "" {
			return view, challengerID, targetID, animal
		}
		active := room.clients[view.TurnPlayerID]
		opened := active.request(t, fmt.Sprintf("%s-auction-%d", prefix, turn), "turn.auction", nil)
		closed := active.request(t, fmt.Sprintf("%s-close-%d", prefix, turn), "auction.close", nil)
		if opened.Type != "snapshot" || closed.Type != "snapshot" {
			t.Fatalf("auction %d failed: opened=%+v closed=%+v", turn, opened, closed)
		}
		view = closed.Game.Public
	}
	t.Fatal("no tradable animal found")
	return game.PublicView{}, "", "", ""
}

func (room testRoom) syncedRequest(t *testing.T, actorID, id, messageType string, payload any) response {
	t.Helper()
	response := room.clients[actorID].request(t, id, messageType, payload)
	if response.Type != "snapshot" {
		t.Fatalf("%s response = %+v", messageType, response)
	}
	for playerID, client := range room.clients {
		if playerID != actorID {
			client.awaitVersion(t, response.Game.Version)
		}
	}
	return response
}

type testClient struct {
	connection *websocket.Conn
}

type response struct {
	Type      string         `json:"type"`
	RequestID string         `json:"requestId"`
	Session   *session       `json:"session,omitempty"`
	Game      game.Snapshot  `json:"game"`
	Error     *responseError `json:"error,omitempty"`
}

type session struct {
	GameID   string `json:"gameId"`
	PlayerID string `json:"playerId"`
	Token    string `json:"token"`
}

type responseError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func newClient(t *testing.T, url string) *testClient {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { connection.CloseNow() })
	return &testClient{connection: connection}
}

func (client *testClient) request(t *testing.T, id, messageType string, payload any) response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	response, err := client.do(ctx, id, messageType, payload)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func (client *testClient) do(ctx context.Context, id, messageType string, payload any) (response, error) {
	message := map[string]any{"id": id, "type": messageType, "payload": payload}
	if err := wsjson.Write(ctx, client.connection, message); err != nil {
		return response{}, err
	}
	for {
		var response response
		if err := wsjson.Read(ctx, client.connection, &response); err != nil {
			return response, err
		}
		if response.RequestID == id {
			return response, nil
		}
	}
}

func (client *testClient) awaitVersion(t *testing.T, version uint64) response {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		var response response
		if err := wsjson.Read(ctx, client.connection, &response); err != nil {
			t.Fatal(err)
		}
		if response.Type == "snapshot" && response.Game.Version >= version {
			return response
		}
	}
}

func animalCount(public game.PublicView, playerID string, animal game.Animal) int {
	for _, player := range public.Players {
		if player.ID == playerID {
			return player.Animals[animal]
		}
	}
	return 0
}

func findTrade(public game.PublicView) (string, string, game.Animal) {
	active := public.TurnPlayerID
	for _, challenger := range public.Players {
		if challenger.ID != active {
			continue
		}
		for animal, count := range challenger.Animals {
			if count == 0 || count == 4 {
				continue
			}
			for _, target := range public.Players {
				if target.ID != active && target.Animals[animal] > 0 && target.Animals[animal] < 4 {
					return active, target.ID, animal
				}
			}
		}
	}
	return "", "", ""
}
