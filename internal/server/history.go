package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/khoi/kuhhandel/internal/game"
)

var errGameNotFound = errors.New("game not found")

type historyPlayer struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type historySummary struct {
	ID         string          `json:"id"`
	Status     game.Status     `json:"status"`
	Phase      game.Phase      `json:"phase"`
	Players    []historyPlayer `json:"players"`
	WinnerIDs  []string        `json:"winnerIds,omitempty"`
	EventCount int             `json:"eventCount"`
	CreatedAt  time.Time       `json:"createdAt"`
	UpdatedAt  time.Time       `json:"updatedAt"`
}

type historyFrame struct {
	Type       string          `json:"type"`
	ActorID    string          `json:"actorId,omitempty"`
	OccurredAt time.Time       `json:"occurredAt"`
	Public     game.PublicView `json:"public"`
}

type loadedHistory struct {
	Frames     []historyFrame
	Public     game.PublicView
	EventCount int
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func (server *Server) serveHistoryList(writer http.ResponseWriter, request *http.Request) {
	offset, err := historyOffset(request)
	if err != nil {
		writeAPIError(writer, http.StatusBadRequest, "invalid offset")
		return
	}
	gameIDs, hasMore, err := server.events.RecentGameIDs(request.Context(), 30, offset)
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "could not load games")
		return
	}
	summaries := make([]historySummary, 0, len(gameIDs))
	for _, gameID := range gameIDs {
		history, err := server.loadHistory(request.Context(), gameID, false)
		if err != nil {
			writeAPIError(writer, http.StatusInternalServerError, "could not load games")
			return
		}
		players := make([]historyPlayer, len(history.Public.Players))
		for index, player := range history.Public.Players {
			players[index] = historyPlayer{ID: player.ID, Name: player.Name}
		}
		summaries = append(summaries, historySummary{
			ID:         gameID,
			Status:     history.Public.Status,
			Phase:      history.Public.Phase,
			Players:    players,
			WinnerIDs:  history.Public.WinnerIDs,
			EventCount: history.EventCount,
			CreatedAt:  history.CreatedAt,
			UpdatedAt:  history.UpdatedAt,
		})
	}
	writeJSON(writer, http.StatusOK, struct {
		Games   []historySummary `json:"games"`
		HasMore bool             `json:"hasMore"`
	}{Games: summaries, HasMore: hasMore})
}

func (server *Server) serveHistoryReplay(writer http.ResponseWriter, request *http.Request) {
	gameID := request.PathValue("gameID")
	history, err := server.loadHistory(request.Context(), gameID, true)
	if errors.Is(err, errGameNotFound) {
		writeAPIError(writer, http.StatusNotFound, "game not found")
		return
	}
	if err != nil {
		writeAPIError(writer, http.StatusInternalServerError, "could not load game")
		return
	}
	writeJSON(writer, http.StatusOK, struct {
		Frames []historyFrame `json:"frames"`
	}{Frames: history.Frames})
}

func (server *Server) loadHistory(ctx context.Context, gameID string, captureFrames bool) (loadedHistory, error) {
	events, err := server.events.Load(ctx, gameID)
	if err != nil {
		return loadedHistory{}, err
	}
	if len(events) == 0 {
		return loadedHistory{}, errGameNotFound
	}
	aggregate := game.New(gameID)
	var frames []historyFrame
	if captureFrames {
		frames = make([]historyFrame, 0, len(events))
	}
	for _, event := range events {
		if err := aggregate.Apply(event); err != nil {
			return loadedHistory{}, err
		}
		if captureFrames {
			frames = append(frames, historyFrame{
				Type:       event.Type,
				ActorID:    event.ActorID,
				OccurredAt: event.OccurredAt,
				Public:     aggregate.Public(),
			})
		}
	}
	public := aggregate.Public()
	if captureFrames {
		public = frames[len(frames)-1].Public
	}
	return loadedHistory{
		Frames:     frames,
		Public:     public,
		EventCount: len(events),
		CreatedAt:  events[0].OccurredAt,
		UpdatedAt:  events[len(events)-1].OccurredAt,
	}, nil
}

func historyOffset(request *http.Request) (int, error) {
	values, found := request.URL.Query()["offset"]
	if !found {
		return 0, nil
	}
	if len(values) != 1 {
		return 0, errors.New("offset must appear once")
	}
	offset, err := strconv.Atoi(values[0])
	if err != nil || offset < 0 || offset > 1_000_000 {
		return 0, errors.New("invalid offset")
	}
	return offset, nil
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Cache-Control", "no-store")
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.Header().Set("X-Content-Type-Options", "nosniff")
	writer.WriteHeader(status)
	json.NewEncoder(writer).Encode(value)
}

func writeAPIError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, struct {
		Error string `json:"error"`
	}{Error: message})
}
