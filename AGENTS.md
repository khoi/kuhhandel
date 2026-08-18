# Repository Guidelines

## Project Structure & Module Organization

`cmd/kuhhandel` contains the Go server entry point. Core rules and state live in `internal/game`; WebSocket handling, protocol checks, and the public replay archive live in `internal/server`; SQLite persistence lives in `internal/store`. Keep Go tests beside their packages as `*_test.go`.

The Python client is a separate package under `sdk/python`. Runtime code belongs in `sdk/python/src/kuhhandel`, with tests in `sdk/python/tests`. `rule.md` records the board-game rules that guide engine behavior.

## Build, Test, and Development Commands

- `go run ./cmd/kuhhandel -addr :8080 -db kuhhandel.db` starts the server, WebSocket endpoint, and replay archive.
- `go test ./...` runs all Go tests.
- `go test -race ./...` checks concurrent game and connection code.
- `go vet ./...` checks common Go mistakes.
- `python -m pip install -e ./sdk/python` installs the SDK for local work.
- `cd sdk/python && python -m unittest discover -v` runs Python unit and live-server tests.
- `ruff check sdk/python` and `mypy --strict sdk/python/src/kuhhandel` check Python style and types.

## Coding Style & Naming Conventions

Run `gofmt` on Go files. Use short package names, exported `PascalCase` names, and internal `camelCase` names. Python uses four spaces, `snake_case` functions, `PascalCase` classes, immutable data models, and the Ruff rules in `pyproject.toml`.

Keep the server authoritative. Clients may render public state and their own private view, but must not predict or settle game state. Avoid duplicate state, unused code, hidden fallbacks, and protocol fields that clients can derive.

## Testing Guidelines

Use Go's `testing` package and Python's `unittest`. Name Go tests `TestBehavior` and Python tests `test_behavior`. Add a regression test for each bug. Cover rejected moves, privacy, replay after restart, concurrent requests, and malformed input when changing rules or protocol code. Run race tests for connection or runtime changes.

## Commit & Pull Request Guidelines

Use a short imperative subject, such as `Build Python client SDK`; add a scope when useful, as in `history: add public game replay archive`. Keep commits focused. Pull requests should explain behavior and protocol or schema effects, list verification commands, link relevant issues, and include screenshots for archive UI changes.

## Security & Configuration

Never commit SQLite databases, session tokens, or local credentials. Keep private money, offers, shuffle seeds, and command records out of public history responses. Preserve request-size, rate, origin, turn, ownership, and session checks. Use TLS in production.
