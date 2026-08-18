# Kuhhandel server

An authoritative server for classic second-edition Kuhhandel in Go. Clients receive public game state and their own money. SQLite stores every accepted game event. Restarting the server replays each game from its log.

## Run

```sh
go run ./cmd/kuhhandel -addr :8080 -db kuhhandel.db
```

Health check: `GET /health`

WebSocket: `GET /ws`

Python clients can use the typed async [SDK](sdk/python/README.md).

## Messages

Send:

```json
{"id":"1","type":"room.create","payload":{"name":"Alice"}}
```

The server replies with either a `snapshot` or `error`. A create, join, or resume reply also has a session with `gameId`, `playerId`, and `token`. Store that session and use it to reconnect.

| Type | Payload |
| --- | --- |
| `room.create` | `{"name":"Alice"}` |
| `room.join` | `{"gameId":"game_...","name":"Bob"}` |
| `session.resume` | `{"gameId":"game_...","playerId":"player_...","token":"session_..."}` |
| `game.start` | `null` |
| `turn.auction` | `null` |
| `auction.bid` | `{"amount":20,"payment":{"10":2}}` |
| `auction.close` | `null` |
| `auction.resolve` | `{"buy":false}` or `{"buy":true,"payment":{"50":1}}` |
| `turn.trade` | `{"targetId":"player_...","animal":"pig","offer":{"0":1,"10":1}}` |
| `trade.accept` | `null` |
| `trade.counter` | `{"offer":{"50":1}}` |
| `trade.reoffer` | `{"offer":{"0":1}}` |

Money keys are `0`, `10`, `50`, `100`, `200`, and `500`. Values are card counts.

The first player is the host. The host can start once three to five players have joined. The server checks every turn, bid, payment, trade, and settlement. Clients should render `game.public`, `game.self`, and `game.self.legalActions` without keeping their own game state.

## Security limits

- The server binds each socket to one authenticated player and revokes the older socket when that session reconnects.
- Event logs contain hashes of session tokens, not bearer tokens.
- Plain SQLite paths use owner-only file permissions.
- The server accepts text JSON messages up to 8 KiB. It rejects binary messages, extra fields, duplicate fields, unsafe request IDs, and payloads on commands that take none.
- Each socket can send at most 64 messages per one-second window. Slow clients are disconnected when their outbound queue fills.
- Request IDs cannot be reused for accepted commands, including after a restart.
- Browser WebSocket connections must use the server's origin.
- Production deployments must serve the endpoint through TLS.
- The rules engine checks the actor, host, turn, phase, animal ownership, money ownership, bid order, trade target, and settlement before it writes an event.

## Verify

```sh
go test ./...
go test -race ./...
go vet ./...
```
