from typing import Any, Optional


def money(ten: int = 4) -> dict[str, int]:
    return {"0": 2, "10": ten, "50": 1, "100": 0, "200": 0, "500": 0}


def snapshot(
    version: int = 1, game_id: str = "game_one", player_id: str = "player_one", phase: str = "lobby"
) -> dict[str, Any]:
    status = "lobby" if phase == "lobby" else "playing"
    value: dict[str, Any] = {
        "version": version,
        "public": {
            "gameId": game_id,
            "status": status,
            "phase": phase,
            "hostId": "player_one",
            "players": [
                {"id": "player_one", "name": "Alice", "seat": 0, "animals": {"pig": 1}, "score": 0},
                {"id": "player_two", "name": "Bob", "seat": 1, "animals": {}, "score": 0},
            ],
            "deckRemaining": 40,
        },
        "self": {"playerId": player_id, "money": money(), "legalActions": []},
    }
    return value


def response(request_id: Optional[str], version: int = 1, session: bool = False) -> dict[str, Any]:
    value: dict[str, Any] = {"type": "snapshot", "game": snapshot(version)}
    if request_id is not None:
        value["requestId"] = request_id
    if session:
        value["session"] = {"gameId": "game_one", "playerId": "player_one", "token": "session_secret"}
    return value
