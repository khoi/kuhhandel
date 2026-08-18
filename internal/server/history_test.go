package server_test

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/khoi/kuhhandel/internal/game"
)

func TestHistoryListsAndReplaysOnlyPublicState(t *testing.T) {
	_, httpServer, webSocketURL := newTestServer(t, t.TempDir()+"/games.db")
	room := newRoom(t, webSocketURL)
	room.start(t)
	room.host.request(t, "private-auction-command", "turn.auction", nil)
	bid := room.bob.request(t, "private-bid-command", "auction.bid", map[string]any{"amount": 10, "payment": game.Money{Ten: 1}})

	listBody, listResponse := getHTTPBody(t, httpServer.URL+"/api/history")
	if listResponse.StatusCode != http.StatusOK || listResponse.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("history list status=%d cache=%q", listResponse.StatusCode, listResponse.Header.Get("Cache-Control"))
	}
	var list struct {
		Games []struct {
			ID         string `json:"id"`
			Status     string `json:"status"`
			EventCount int    `json:"eventCount"`
			Players    []struct {
				Name string `json:"name"`
			} `json:"players"`
		} `json:"games"`
		HasMore bool `json:"hasMore"`
	}
	if err := json.Unmarshal(listBody, &list); err != nil {
		t.Fatal(err)
	}
	if len(list.Games) != 1 || list.Games[0].ID != room.hostSession.GameID || list.Games[0].Status != "playing" || list.Games[0].EventCount != 6 || list.HasMore {
		t.Fatalf("history list = %+v", list)
	}
	if len(list.Games[0].Players) != 3 || list.Games[0].Players[0].Name != "Alice" {
		t.Fatalf("history players = %+v", list.Games[0].Players)
	}

	replayBody, replayResponse := getHTTPBody(t, httpServer.URL+"/api/history/"+room.hostSession.GameID)
	if replayResponse.StatusCode != http.StatusOK {
		t.Fatalf("replay status = %d, body = %s", replayResponse.StatusCode, replayBody)
	}
	var replay struct {
		Frames []struct {
			Type       string          `json:"type"`
			ActorID    string          `json:"actorId"`
			OccurredAt time.Time       `json:"occurredAt"`
			Public     game.PublicView `json:"public"`
		} `json:"frames"`
	}
	if err := json.Unmarshal(replayBody, &replay); err != nil {
		t.Fatal(err)
	}
	if len(replay.Frames) != 6 {
		t.Fatalf("frames = %d", len(replay.Frames))
	}
	last := replay.Frames[len(replay.Frames)-1]
	if last.Type != "auction.bid_placed" || last.ActorID != room.bobSession.PlayerID || last.OccurredAt.IsZero() {
		t.Fatalf("last frame = %+v", last)
	}
	if last.Public.Auction == nil || last.Public.Auction.HighestBid != bid.Game.Public.Auction.HighestBid {
		t.Fatalf("public auction = %+v", last.Public.Auction)
	}
	encoded := string(replayBody)
	for _, private := range []string{room.hostSession.Token, "private-bid-command", `"authHash"`, `"commandId"`, `"data"`, `"payment"`, `"offer"`, `"seed"`, `"money"`} {
		if strings.Contains(encoded, private) {
			t.Fatalf("replay leaked %q: %s", private, encoded)
		}
	}
}

func TestHistoryRoutesRejectBadRequestsAndServeLockedDownAssets(t *testing.T) {
	_, httpServer, _ := newTestServer(t, t.TempDir()+"/games.db")

	for _, path := range []string{"/api/history?offset=-1", "/api/history?offset=1&offset=2", "/api/history?offset=1000001"} {
		body, response := getHTTPBody(t, httpServer.URL+path)
		if response.StatusCode != http.StatusBadRequest || response.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Fatalf("%s status=%d content-type=%q body=%s", path, response.StatusCode, response.Header.Get("Content-Type"), body)
		}
	}

	body, response := getHTTPBody(t, httpServer.URL+"/api/history/missing")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("missing replay status=%d body=%s", response.StatusCode, body)
	}

	body, response = getHTTPBody(t, httpServer.URL+"/")
	if response.StatusCode != http.StatusOK || response.Header.Get("Content-Type") != "text/html; charset=utf-8" || !strings.Contains(response.Header.Get("Content-Security-Policy"), "default-src 'none'") || len(body) == 0 {
		t.Fatalf("page status=%d headers=%v body length=%d", response.StatusCode, response.Header, len(body))
	}

	for _, path := range []string{"/app.css", "/app.js"} {
		asset, assetResponse := getHTTPBody(t, httpServer.URL+path)
		if assetResponse.StatusCode != http.StatusOK || assetResponse.Header.Get("X-Content-Type-Options") != "nosniff" || len(asset) == 0 {
			t.Fatalf("%s status=%d headers=%v body length=%d", path, assetResponse.StatusCode, assetResponse.Header, len(asset))
		}
	}

	request, err := http.NewRequest(http.MethodPost, httpServer.URL+"/api/history", nil)
	if err != nil {
		t.Fatal(err)
	}
	postResponse, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	postResponse.Body.Close()
	if postResponse.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("post status = %d", postResponse.StatusCode)
	}

	_, response = getHTTPBody(t, httpServer.URL+"/not-a-route")
	if response.StatusCode != http.StatusNotFound {
		t.Fatalf("unknown route status = %d", response.StatusCode)
	}
}

func getHTTPBody(t *testing.T, url string) ([]byte, *http.Response) {
	t.Helper()
	response, err := http.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return body, response
}
