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
