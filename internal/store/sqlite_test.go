package store_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/khoi/kuhhandel/internal/game"
	"github.com/khoi/kuhhandel/internal/store"
)

func TestRecentGameIDsPaginatesNewestFirst(t *testing.T) {
	database, err := store.Open(t.TempDir() + "/games.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	start := time.Date(2026, time.August, 18, 12, 0, 0, 0, time.UTC)
	for index := range 31 {
		gameID := fmt.Sprintf("game_%02d", index)
		err := database.Append(context.Background(), gameID, game.Event{
			Version:    1,
			Type:       "room.created",
			Data:       []byte(`{}`),
			ActorID:    "player",
			CommandID:  "create",
			OccurredAt: start.Add(time.Duration(index) * time.Minute),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	first, hasMore, err := database.RecentGameIDs(context.Background(), 30, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(first) != 30 || !hasMore || first[0] != "game_30" || first[29] != "game_01" {
		t.Fatalf("first page=%v hasMore=%t", first, hasMore)
	}
	second, hasMore, err := database.RecentGameIDs(context.Background(), 30, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(second) != 1 || hasMore || second[0] != "game_00" {
		t.Fatalf("second page=%v hasMore=%t", second, hasMore)
	}
}
