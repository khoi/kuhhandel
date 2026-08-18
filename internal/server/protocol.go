package server

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/khoi/kuhhandel/internal/game"
	"github.com/khoi/kuhhandel/internal/store"
)

type request struct {
	ID      string          `json:"id"`
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type response struct {
	Type      string         `json:"type"`
	RequestID string         `json:"requestId,omitempty"`
	Session   *session       `json:"session,omitempty"`
	Game      *game.Snapshot `json:"game,omitempty"`
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

func (server *Server) handle(ctx context.Context, peer *client, current *session, message request) (*session, error) {
	switch message.Type {
	case "room.create":
		if current != nil {
			return nil, protocolError("already_attached", "connection already has a player")
		}
		var payload struct {
			Name string `json:"name"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		identity, token, err := newIdentity(payload.Name)
		if err != nil {
			return nil, err
		}
		gameID, err := randomID("game_", 12)
		if err != nil {
			return nil, err
		}
		runtime := newGameRuntime(gameID, server.events, game.New(gameID))
		snapshot, err := runtime.execute(ctx, identity.ID, identity.ID+":"+message.ID, game.CreateRoom{Player: identity})
		if err != nil {
			runtime.cancel()
			<-runtime.done
			return nil, err
		}
		server.mu.Lock()
		server.games[gameID] = runtime
		server.mu.Unlock()
		created := &session{GameID: gameID, PlayerID: identity.ID, Token: token}
		server.attach(peer, created)
		server.publish(ctx, gameID, peer, message.ID, created, &snapshot)
		return created, nil
	case "room.join":
		if current != nil {
			return nil, protocolError("already_attached", "connection already has a player")
		}
		var payload struct {
			GameID string `json:"gameId"`
			Name   string `json:"name"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		runtime := server.runtime(payload.GameID)
		if runtime == nil {
			return nil, protocolError("game_not_found", "game does not exist")
		}
		identity, token, err := newIdentity(payload.Name)
		if err != nil {
			return nil, err
		}
		snapshot, err := runtime.execute(ctx, identity.ID, identity.ID+":"+message.ID, game.JoinRoom{Player: identity})
		if err != nil {
			return nil, err
		}
		joined := &session{GameID: payload.GameID, PlayerID: identity.ID, Token: token}
		server.attach(peer, joined)
		server.publish(ctx, payload.GameID, peer, message.ID, joined, &snapshot)
		return joined, nil
	case "session.resume":
		if current != nil {
			return nil, protocolError("already_attached", "connection already has a player")
		}
		var payload session
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		runtime := server.runtime(payload.GameID)
		if runtime == nil {
			return nil, protocolError("unauthorized", "session is invalid")
		}
		authenticated, err := runtime.authenticate(ctx, payload.PlayerID, payload.Token)
		if err != nil {
			return nil, err
		}
		if !authenticated {
			return nil, protocolError("unauthorized", "session is invalid")
		}
		resumed := &session{GameID: payload.GameID, PlayerID: payload.PlayerID, Token: payload.Token}
		server.attach(peer, resumed)
		server.publish(ctx, payload.GameID, peer, message.ID, resumed, nil)
		return resumed, nil
	case "game.start":
		if err := requireNoPayload(message.Payload); err != nil {
			return nil, err
		}
		seed, err := randomSeed()
		if err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.StartGame{Seed: seed})
	case "turn.auction":
		if err := requireNoPayload(message.Payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.BeginAuction{})
	case "auction.bid":
		var payload struct {
			Amount  int        `json:"amount"`
			Payment game.Money `json:"payment"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.PlaceBid{Amount: payload.Amount, Payment: payload.Payment})
	case "auction.close":
		if err := requireNoPayload(message.Payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.CloseAuction{})
	case "auction.resolve":
		var payload struct {
			Buy     bool       `json:"buy"`
			Payment game.Money `json:"payment"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.ResolveAuction{Buy: payload.Buy, Payment: payload.Payment})
	case "turn.trade":
		var payload struct {
			TargetID string      `json:"targetId"`
			Animal   game.Animal `json:"animal"`
			Offer    game.Money  `json:"offer"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.BeginTrade{TargetID: payload.TargetID, Animal: payload.Animal, Offer: payload.Offer})
	case "trade.accept":
		if err := requireNoPayload(message.Payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.AcceptTrade{})
	case "trade.counter":
		var payload struct {
			Offer game.Money `json:"offer"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.CounterTrade{Offer: payload.Offer})
	case "trade.reoffer":
		var payload struct {
			Offer game.Money `json:"offer"`
		}
		if err := decodePayload(message.Payload, &payload); err != nil {
			return nil, err
		}
		return nil, server.execute(ctx, peer, current, message.ID, game.ReofferTrade{Offer: payload.Offer})
	default:
		return nil, protocolError("unknown_message", "unknown message type")
	}
}

func (server *Server) execute(ctx context.Context, peer *client, current *session, commandID string, command game.Command) error {
	if current == nil {
		return protocolError("unauthorized", "join a game first")
	}
	runtime := server.runtime(current.GameID)
	if runtime == nil {
		return protocolError("game_not_found", "game does not exist")
	}
	snapshot, err := runtime.execute(ctx, current.PlayerID, current.PlayerID+":"+commandID, command)
	if err != nil {
		return err
	}
	server.publish(ctx, current.GameID, peer, commandID, nil, &snapshot)
	return nil
}

func responseForError(requestID string, err error) response {
	var rule *game.RuleError
	var protocol *responseError
	switch {
	case errors.As(err, &rule):
		return errorResponse(requestID, rule.Code, rule.Message)
	case errors.As(err, &protocol):
		return errorResponse(requestID, protocol.Code, protocol.Message)
	case errors.Is(err, store.ErrDuplicateCommand):
		return errorResponse(requestID, "duplicate_request", "request id was already used")
	default:
		return errorResponse(requestID, "internal", "internal server error")
	}
}

func errorResponse(requestID, code, message string) response {
	return response{Type: "error", RequestID: requestID, Error: &responseError{Code: code, Message: message}}
}

func protocolError(code, message string) *responseError {
	return &responseError{Code: code, Message: message}
}

func (problem *responseError) Error() string {
	return problem.Message
}
