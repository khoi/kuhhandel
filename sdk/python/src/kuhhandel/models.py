from __future__ import annotations

from collections.abc import Mapping, Sequence
from dataclasses import dataclass, field
from enum import Enum
from types import MappingProxyType
from typing import Any, TypeVar

from .errors import ProtocolError


class Animal(str, Enum):
    ROOSTER = "rooster"
    GOOSE = "goose"
    CAT = "cat"
    DOG = "dog"
    SHEEP = "sheep"
    GOAT = "goat"
    DONKEY = "donkey"
    PIG = "pig"
    COW = "cow"
    HORSE = "horse"


class Status(str, Enum):
    LOBBY = "lobby"
    PLAYING = "playing"
    FINISHED = "finished"


class Phase(str, Enum):
    LOBBY = "lobby"
    TURN = "turn"
    AUCTION = "auction"
    FIRST_REFUSAL = "first_refusal"
    TRADE_RESPONSE = "trade_response"
    TRADE_REOFFER = "trade_reoffer"
    TRADE_RECOUNTER = "trade_recounter"
    FINISHED = "finished"


class LegalAction(str, Enum):
    BEGIN_AUCTION = "turn.auction"
    PLACE_BID = "auction.bid"
    CLOSE_AUCTION = "auction.close"
    RESOLVE_AUCTION = "auction.resolve"
    BEGIN_TRADE = "turn.trade"
    ACCEPT_TRADE = "trade.accept"
    COUNTER_TRADE = "trade.counter"
    REOFFER_TRADE = "trade.reoffer"


@dataclass(frozen=True)
class Money:
    zero: int = 0
    ten: int = 0
    fifty: int = 0
    hundred: int = 0
    two_hundred: int = 0
    five_hundred: int = 0

    def __post_init__(self) -> None:
        for count in self.counts:
            if isinstance(count, bool) or not isinstance(count, int) or count < 0:
                raise ValueError("money counts must be non-negative integers")

    @property
    def counts(self) -> tuple[int, int, int, int, int, int]:
        return self.zero, self.ten, self.fifty, self.hundred, self.two_hundred, self.five_hundred

    @property
    def total(self) -> int:
        return self.ten * 10 + self.fifty * 50 + self.hundred * 100 + self.two_hundred * 200 + self.five_hundred * 500

    @property
    def card_count(self) -> int:
        return sum(self.counts)

    def to_payload(self) -> dict[str, int]:
        return dict(zip(("0", "10", "50", "100", "200", "500"), self.counts))

    @classmethod
    def from_payload(cls, value: Any, path: str = "money") -> Money:
        data = _object(value, path)
        _fields(data, ("0", "10", "50", "100", "200", "500"), (), path)
        return cls(*(_integer(data[key], f"{path}.{key}") for key in ("0", "10", "50", "100", "200", "500")))


@dataclass(frozen=True)
class Session:
    game_id: str
    player_id: str
    token: str = field(repr=False)

    def to_payload(self) -> dict[str, str]:
        return {"gameId": self.game_id, "playerId": self.player_id, "token": self.token}

    @classmethod
    def from_payload(cls, value: Any, path: str = "session") -> Session:
        data = _object(value, path)
        _fields(data, ("gameId", "playerId", "token"), (), path)
        return cls(
            _text(data["gameId"], f"{path}.gameId"),
            _text(data["playerId"], f"{path}.playerId"),
            _text(data["token"], f"{path}.token"),
        )


@dataclass(frozen=True)
class PublicPlayer:
    id: str
    name: str
    seat: int
    animals: Mapping[Animal, int]
    score: int

    @classmethod
    def from_payload(cls, value: Any, path: str) -> PublicPlayer:
        data = _object(value, path)
        _fields(data, ("id", "name", "seat", "animals", "score"), (), path)
        raw_animals = _object(data["animals"], f"{path}.animals")
        animals: dict[Animal, int] = {}
        for raw_animal, raw_count in raw_animals.items():
            animal = _enum(Animal, raw_animal, f"{path}.animals")
            animals[animal] = _integer(raw_count, f"{path}.animals.{raw_animal}")
        return cls(
            _text(data["id"], f"{path}.id"),
            _text(data["name"], f"{path}.name"),
            _integer(data["seat"], f"{path}.seat"),
            MappingProxyType(animals),
            _integer(data["score"], f"{path}.score"),
        )


@dataclass(frozen=True)
class Auction:
    animal: Animal
    auctioneer_id: str
    highest_bid: int
    highest_bidder_id: str | None

    @classmethod
    def from_payload(cls, value: Any, path: str = "game.public.auction") -> Auction:
        data = _object(value, path)
        _fields(data, ("animal", "auctioneerId", "highestBid"), ("highestBidderId",), path)
        return cls(
            _enum(Animal, data["animal"], f"{path}.animal"),
            _text(data["auctioneerId"], f"{path}.auctioneerId"),
            _integer(data["highestBid"], f"{path}.highestBid"),
            _optional_text(data.get("highestBidderId"), f"{path}.highestBidderId"),
        )


@dataclass(frozen=True)
class Trade:
    challenger_id: str
    target_id: str
    animal: Animal
    card_count: int

    @classmethod
    def from_payload(cls, value: Any, path: str = "game.public.trade") -> Trade:
        data = _object(value, path)
        _fields(data, ("challengerId", "targetId", "animal", "cardCount"), (), path)
        return cls(
            _text(data["challengerId"], f"{path}.challengerId"),
            _text(data["targetId"], f"{path}.targetId"),
            _enum(Animal, data["animal"], f"{path}.animal"),
            _integer(data["cardCount"], f"{path}.cardCount"),
        )


@dataclass(frozen=True)
class PublicGame:
    game_id: str
    status: Status
    phase: Phase
    host_id: str
    turn_player_id: str | None
    players: tuple[PublicPlayer, ...]
    deck_remaining: int
    auction: Auction | None
    trade: Trade | None
    winner_ids: tuple[str, ...]

    @classmethod
    def from_payload(cls, value: Any, path: str = "game.public") -> PublicGame:
        data = _object(value, path)
        _fields(
            data,
            ("gameId", "status", "phase", "hostId", "players", "deckRemaining"),
            ("turnPlayerId", "auction", "trade", "winnerIds"),
            path,
        )
        players = _sequence(data["players"], f"{path}.players")
        winner_ids = _sequence(data.get("winnerIds", []), f"{path}.winnerIds")
        auction = Auction.from_payload(data["auction"], f"{path}.auction") if "auction" in data else None
        trade = Trade.from_payload(data["trade"], f"{path}.trade") if "trade" in data else None
        return cls(
            _text(data["gameId"], f"{path}.gameId"),
            _enum(Status, data["status"], f"{path}.status"),
            _enum(Phase, data["phase"], f"{path}.phase"),
            _text(data["hostId"], f"{path}.hostId"),
            _optional_text(data.get("turnPlayerId"), f"{path}.turnPlayerId"),
            tuple(
                PublicPlayer.from_payload(player, f"{path}.players[{index}]") for index, player in enumerate(players)
            ),
            _integer(data["deckRemaining"], f"{path}.deckRemaining"),
            auction,
            trade,
            tuple(_text(player_id, f"{path}.winnerIds[{index}]") for index, player_id in enumerate(winner_ids)),
        )


@dataclass(frozen=True)
class SelfView:
    player_id: str
    money: Money
    legal_actions: tuple[LegalAction, ...]
    bid_payment: Money | None
    own_offer: Money | None

    @classmethod
    def from_payload(cls, value: Any, path: str = "game.self") -> SelfView:
        data = _object(value, path)
        _fields(data, ("playerId", "money", "legalActions"), ("bidPayment", "ownOffer"), path)
        actions = _sequence(data["legalActions"], f"{path}.legalActions")
        bid_payment = Money.from_payload(data["bidPayment"], f"{path}.bidPayment") if "bidPayment" in data else None
        own_offer = Money.from_payload(data["ownOffer"], f"{path}.ownOffer") if "ownOffer" in data else None
        return cls(
            _text(data["playerId"], f"{path}.playerId"),
            Money.from_payload(data["money"], f"{path}.money"),
            tuple(_enum(LegalAction, action, f"{path}.legalActions[{index}]") for index, action in enumerate(actions)),
            bid_payment,
            own_offer,
        )


@dataclass(frozen=True)
class Snapshot:
    version: int
    public: PublicGame
    self: SelfView

    @classmethod
    def from_payload(cls, value: Any, path: str = "game") -> Snapshot:
        data = _object(value, path)
        _fields(data, ("version", "public", "self"), (), path)
        return cls(
            _integer(data["version"], f"{path}.version"),
            PublicGame.from_payload(data["public"], f"{path}.public"),
            SelfView.from_payload(data["self"], f"{path}.self"),
        )


EnumType = TypeVar("EnumType", bound=Enum)


def _object(value: Any, path: str) -> dict[str, Any]:
    if not isinstance(value, dict) or any(not isinstance(key, str) for key in value):
        raise ProtocolError(f"{path} must be an object")
    return value


def _sequence(value: Any, path: str) -> Sequence[Any]:
    if not isinstance(value, list):
        raise ProtocolError(f"{path} must be an array")
    return value


def _fields(data: Mapping[str, Any], required: Sequence[str], optional: Sequence[str], path: str) -> None:
    missing = set(required) - data.keys()
    extra = data.keys() - set(required) - set(optional)
    if missing or extra:
        raise ProtocolError(f"{path} has invalid fields")


def _text(value: Any, path: str) -> str:
    if not isinstance(value, str) or not value:
        raise ProtocolError(f"{path} must be non-empty text")
    return value


def _optional_text(value: Any, path: str) -> str | None:
    if value is None:
        return None
    return _text(value, path)


def _integer(value: Any, path: str) -> int:
    if isinstance(value, bool) or not isinstance(value, int) or value < 0:
        raise ProtocolError(f"{path} must be a non-negative integer")
    return int(value)


def _enum(enum_type: type[EnumType], value: Any, path: str) -> EnumType:
    try:
        return enum_type(value)
    except (TypeError, ValueError) as error:
        raise ProtocolError(f"{path} is invalid") from error
