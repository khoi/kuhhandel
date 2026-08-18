package server

import (
	"context"
	"errors"
	"time"

	"github.com/khoi/kuhhandel/internal/game"
	"github.com/khoi/kuhhandel/internal/store"
)

type operation struct {
	actorID   string
	commandID string
	command   game.Command
	playerID  string
	token     string
	auth      bool
	reply     chan operationResult
}

type operationResult struct {
	snapshot      game.Snapshot
	authenticated bool
	err           error
}

type gameRuntime struct {
	gameID     string
	events     *store.SQLite
	operations chan operation
	context    context.Context
	cancel     context.CancelFunc
	done       chan struct{}
}

func newGameRuntime(gameID string, events *store.SQLite, aggregate *game.Aggregate) *gameRuntime {
	runtimeContext, cancel := context.WithCancel(context.Background())
	runtime := &gameRuntime{
		gameID:     gameID,
		events:     events,
		operations: make(chan operation),
		context:    runtimeContext,
		cancel:     cancel,
		done:       make(chan struct{}),
	}
	go runtime.run(aggregate)
	return runtime
}

func (runtime *gameRuntime) run(aggregate *game.Aggregate) {
	defer close(runtime.done)
	for {
		select {
		case operation := <-runtime.operations:
			if operation.auth {
				operation.reply <- operationResult{authenticated: aggregate.Authenticate(operation.playerID, operation.token)}
				continue
			}
			if operation.command != nil {
				event, err := aggregate.Decide(operation.actorID, operation.command)
				if err != nil {
					operation.reply <- operationResult{err: err}
					continue
				}
				event.ActorID = operation.actorID
				event.CommandID = operation.commandID
				event.OccurredAt = time.Now().UTC()
				if err := runtime.events.Append(runtime.context, runtime.gameID, event); err != nil {
					operation.reply <- operationResult{err: err}
					continue
				}
				if err := aggregate.Apply(event); err != nil {
					operation.reply <- operationResult{err: err}
					continue
				}
			}
			snapshot, err := aggregate.Snapshot(operation.playerID)
			operation.reply <- operationResult{snapshot: snapshot, err: err}
		case <-runtime.context.Done():
			return
		}
	}
}

func (runtime *gameRuntime) execute(ctx context.Context, actorID, commandID string, command game.Command) (game.Snapshot, error) {
	return runtime.submit(ctx, operation{actorID: actorID, commandID: commandID, command: command, playerID: actorID})
}

func (runtime *gameRuntime) snapshot(ctx context.Context, playerID string) (game.Snapshot, error) {
	return runtime.submit(ctx, operation{playerID: playerID})
}

func (runtime *gameRuntime) authenticate(ctx context.Context, playerID, token string) (bool, error) {
	result, err := runtime.submitResult(ctx, operation{playerID: playerID, token: token, auth: true})
	return result.authenticated, err
}

func (runtime *gameRuntime) submit(ctx context.Context, operation operation) (game.Snapshot, error) {
	result, err := runtime.submitResult(ctx, operation)
	return result.snapshot, err
}

func (runtime *gameRuntime) submitResult(ctx context.Context, operation operation) (operationResult, error) {
	operation.reply = make(chan operationResult, 1)
	select {
	case runtime.operations <- operation:
	case <-runtime.context.Done():
		return operationResult{}, errors.New("game runtime is closed")
	case <-ctx.Done():
		return operationResult{}, ctx.Err()
	}
	select {
	case result := <-operation.reply:
		return result, result.err
	case <-runtime.context.Done():
		return operationResult{}, errors.New("game runtime is closed")
	case <-ctx.Done():
		return operationResult{}, ctx.Err()
	}
}
