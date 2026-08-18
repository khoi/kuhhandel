package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/khoi/kuhhandel/internal/game"
	_ "modernc.org/sqlite"
)

var ErrConflict = errors.New("event version conflict")
var ErrDuplicateCommand = errors.New("command already handled")

type SQLite struct {
	database *sql.DB
}

func Open(path string) (*SQLite, error) {
	if path != ":memory:" && !strings.HasPrefix(path, "file:") {
		file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
		if err != nil {
			return nil, err
		}
		if err := file.Chmod(0600); err != nil {
			file.Close()
			return nil, err
		}
		if err := file.Close(); err != nil {
			return nil, err
		}
	}
	database, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	database.SetMaxOpenConns(1)
	statements := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		`CREATE TABLE IF NOT EXISTS game_events (
			game_id TEXT NOT NULL,
			version INTEGER NOT NULL,
			type TEXT NOT NULL,
			data BLOB NOT NULL,
			actor_id TEXT NOT NULL,
			command_id TEXT NOT NULL,
			occurred_at TEXT NOT NULL,
			PRIMARY KEY (game_id, version),
			UNIQUE (game_id, command_id)
		)`,
	}
	for _, statement := range statements {
		if _, err := database.Exec(statement); err != nil {
			database.Close()
			return nil, err
		}
	}
	return &SQLite{database: database}, nil
}

func (store *SQLite) Append(ctx context.Context, gameID string, event game.Event) error {
	transaction, err := store.database.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer transaction.Rollback()
	var existing int
	err = transaction.QueryRowContext(ctx, "SELECT 1 FROM game_events WHERE game_id = ? AND command_id = ?", gameID, event.CommandID).Scan(&existing)
	if err == nil {
		return ErrDuplicateCommand
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	var current uint64
	if err := transaction.QueryRowContext(ctx, "SELECT COALESCE(MAX(version), 0) FROM game_events WHERE game_id = ?", gameID).Scan(&current); err != nil {
		return err
	}
	if event.Version != current+1 {
		return ErrConflict
	}
	occurredAt := event.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	_, err = transaction.ExecContext(
		ctx,
		"INSERT INTO game_events (game_id, version, type, data, actor_id, command_id, occurred_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		gameID,
		event.Version,
		event.Type,
		[]byte(event.Data),
		event.ActorID,
		event.CommandID,
		occurredAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		if strings.Contains(err.Error(), "game_events.game_id, game_events.command_id") {
			return ErrDuplicateCommand
		}
		if strings.Contains(err.Error(), "UNIQUE constraint failed") || strings.Contains(err.Error(), "PRIMARY KEY constraint failed") {
			return ErrConflict
		}
		return err
	}
	return transaction.Commit()
}

func (store *SQLite) Load(ctx context.Context, gameID string) ([]game.Event, error) {
	rows, err := store.database.QueryContext(ctx, "SELECT version, type, data, actor_id, command_id, occurred_at FROM game_events WHERE game_id = ? ORDER BY version", gameID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []game.Event{}
	for rows.Next() {
		var event game.Event
		var data []byte
		var occurredAt string
		if err := rows.Scan(&event.Version, &event.Type, &data, &event.ActorID, &event.CommandID, &occurredAt); err != nil {
			return nil, err
		}
		if !json.Valid(data) {
			return nil, fmt.Errorf("game %s event %d has invalid data", gameID, event.Version)
		}
		event.Data = data
		event.OccurredAt, err = time.Parse(time.RFC3339Nano, occurredAt)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (store *SQLite) GameIDs(ctx context.Context) ([]string, error) {
	rows, err := store.database.QueryContext(ctx, "SELECT DISTINCT game_id FROM game_events ORDER BY game_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	gameIDs := []string{}
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, err
		}
		gameIDs = append(gameIDs, gameID)
	}
	return gameIDs, rows.Err()
}

func (store *SQLite) RecentGameIDs(ctx context.Context, limit, offset int) ([]string, bool, error) {
	rows, err := store.database.QueryContext(ctx, `
		SELECT game_id
		FROM game_events
		GROUP BY game_id
		ORDER BY MAX(occurred_at) DESC, game_id
		LIMIT ? OFFSET ?`, limit+1, offset)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	gameIDs := make([]string, 0, limit+1)
	for rows.Next() {
		var gameID string
		if err := rows.Scan(&gameID); err != nil {
			return nil, false, err
		}
		gameIDs = append(gameIDs, gameID)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	hasMore := len(gameIDs) > limit
	if hasMore {
		gameIDs = gameIDs[:limit]
	}
	return gameIDs, hasMore, nil
}

func (store *SQLite) Close() error {
	return store.database.Close()
}
