package server

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/khoi/kuhhandel/internal/game"
	"github.com/khoi/kuhhandel/internal/store"
)

type Server struct {
	events      *store.SQLite
	mu          sync.RWMutex
	games       map[string]*gameRuntime
	connections map[string]map[*client]string
	closeOnce   sync.Once
	closeErr    error
}

type client struct {
	connection *websocket.Conn
	context    context.Context
	cancel     context.CancelFunc
	send       chan response
}

func New(databasePath string) (*Server, error) {
	events, err := store.Open(databasePath)
	if err != nil {
		return nil, err
	}
	server := &Server{
		events:      events,
		games:       map[string]*gameRuntime{},
		connections: map[string]map[*client]string{},
	}
	gameIDs, err := events.GameIDs(context.Background())
	if err != nil {
		events.Close()
		return nil, err
	}
	for _, gameID := range gameIDs {
		history, err := events.Load(context.Background(), gameID)
		if err != nil {
			server.Close()
			return nil, err
		}
		aggregate := game.New(gameID)
		for _, event := range history {
			if err := aggregate.Apply(event); err != nil {
				server.Close()
				return nil, fmt.Errorf("replay game %s: %w", gameID, err)
			}
		}
		server.games[gameID] = newGameRuntime(gameID, events, aggregate)
	}
	return server, nil
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /ws", server.serveWebSocket)
	return mux
}

func (server *Server) Close() error {
	server.closeOnce.Do(func() {
		server.mu.Lock()
		runtimes := make([]*gameRuntime, 0, len(server.games))
		for _, runtime := range server.games {
			runtimes = append(runtimes, runtime)
			runtime.cancel()
		}
		for _, connections := range server.connections {
			for peer := range connections {
				peer.cancel()
			}
		}
		server.mu.Unlock()
		for _, runtime := range runtimes {
			<-runtime.done
		}
		server.closeErr = server.events.Close()
	})
	return server.closeErr
}

func (server *Server) serveWebSocket(writer http.ResponseWriter, httpRequest *http.Request) {
	connection, err := websocket.Accept(writer, httpRequest, nil)
	if err != nil {
		return
	}
	connection.SetReadLimit(64 << 10)
	clientContext, cancel := context.WithCancel(context.Background())
	peer := &client{connection: connection, context: clientContext, cancel: cancel, send: make(chan response, 32)}
	go peer.write()
	defer func() {
		cancel()
		server.detach(peer)
		connection.CloseNow()
	}()
	var currentSession *session
	for {
		var message request
		if err := wsjson.Read(clientContext, connection, &message); err != nil {
			return
		}
		if strings.TrimSpace(message.ID) == "" {
			peer.enqueue(errorResponse("", "invalid_request", "request id is required"))
			continue
		}
		createdSession, err := server.handle(clientContext, peer, currentSession, message)
		if createdSession != nil {
			currentSession = createdSession
		}
		if err != nil {
			peer.enqueue(responseForError(message.ID, err))
		}
	}
}

func (server *Server) runtime(gameID string) *gameRuntime {
	server.mu.RLock()
	defer server.mu.RUnlock()
	return server.games[gameID]
}

func (server *Server) attach(peer *client, current *session) {
	server.mu.Lock()
	defer server.mu.Unlock()
	if server.connections[current.GameID] == nil {
		server.connections[current.GameID] = map[*client]string{}
	}
	server.connections[current.GameID][peer] = current.PlayerID
}

func (server *Server) detach(peer *client) {
	server.mu.Lock()
	defer server.mu.Unlock()
	for gameID, connections := range server.connections {
		delete(connections, peer)
		if len(connections) == 0 {
			delete(server.connections, gameID)
		}
	}
}

func (server *Server) publish(ctx context.Context, gameID string, requester *client, requestID string, current *session, requesterSnapshot *game.Snapshot) {
	server.mu.RLock()
	runtime := server.games[gameID]
	type recipient struct {
		peer     *client
		playerID string
	}
	recipients := make([]recipient, 0, len(server.connections[gameID]))
	for peer, playerID := range server.connections[gameID] {
		recipients = append(recipients, recipient{peer: peer, playerID: playerID})
	}
	server.mu.RUnlock()
	for _, recipient := range recipients {
		var snapshot game.Snapshot
		if recipient.peer == requester && requesterSnapshot != nil {
			snapshot = *requesterSnapshot
		} else {
			var err error
			snapshot, err = runtime.snapshot(ctx, recipient.playerID)
			if err != nil {
				recipient.peer.enqueue(responseForError("", err))
				continue
			}
		}
		message := response{Type: "snapshot", Game: &snapshot}
		if recipient.peer == requester {
			message.RequestID = requestID
			message.Session = current
		}
		recipient.peer.enqueue(message)
	}
}

func (peer *client) enqueue(message response) {
	select {
	case peer.send <- message:
	case <-peer.context.Done():
	default:
		peer.cancel()
	}
}

func (peer *client) write() {
	for {
		select {
		case message := <-peer.send:
			if err := wsjson.Write(peer.context, peer.connection, message); err != nil {
				peer.cancel()
				return
			}
		case <-peer.context.Done():
			return
		}
	}
}
