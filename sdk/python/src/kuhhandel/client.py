from __future__ import annotations

import asyncio
import json
from collections.abc import AsyncIterator
from dataclasses import dataclass
from typing import Any
from uuid import uuid4

from websockets.asyncio.client import ClientConnection, connect

from .errors import ClientStateError, ConnectionLost, KuhhandelError, ProtocolError, RequestTimeout, ServerError
from .models import Animal, Money, Session, Snapshot, _fields, _object, _optional_text, _text


@dataclass(frozen=True)
class _SnapshotMessage:
    request_id: str | None
    session: Session | None
    snapshot: Snapshot


@dataclass(frozen=True)
class _ErrorMessage:
    request_id: str | None
    error: ServerError


class Client:
    def __init__(self, url: str, timeout: float = 10) -> None:
        if not isinstance(url, str) or not url.startswith(("ws://", "wss://")):
            raise ValueError("url must use ws:// or wss://")
        if isinstance(timeout, bool) or not isinstance(timeout, (int, float)) or timeout <= 0:
            raise ValueError("timeout must be positive")
        self._url = url
        self._timeout = timeout
        self._connection: ClientConnection | None = None
        self._reader: asyncio.Task[None] | None = None
        self._send_lock = asyncio.Lock()
        self._pending: dict[str, asyncio.Future[Snapshot]] = {}
        self._subscribers: set[asyncio.Queue[Snapshot | KuhhandelError]] = set()
        self._session: Session | None = None
        self._snapshot: Snapshot | None = None

    @property
    def url(self) -> str:
        return self._url

    @property
    def connected(self) -> bool:
        return self._connection is not None and self._reader is not None and not self._reader.done()

    @property
    def session(self) -> Session | None:
        return self._session

    @property
    def snapshot(self) -> Snapshot | None:
        return self._snapshot

    async def __aenter__(self) -> Client:
        return await self.connect()

    async def __aexit__(self, exception_type: Any, exception: Any, traceback: Any) -> None:
        await self.close()

    async def connect(self) -> Client:
        if self.connected:
            raise ClientStateError("client is already connected")
        try:
            connection = await connect(
                self._url,
                open_timeout=self._timeout,
                close_timeout=self._timeout,
                ping_interval=20,
                ping_timeout=20,
                max_size=8 << 10,
                max_queue=16,
            )
        except Exception as cause:
            raise ConnectionLost(f"could not connect: {cause}") from cause
        self._connection = connection
        self._reader = asyncio.create_task(self._read(connection))
        return self

    async def close(self) -> None:
        connection = self._connection
        reader = self._reader
        self._connection = None
        self._reader = None
        error = ConnectionLost("client is closed")
        self._fail_pending(error)
        self._publish_error(error)
        if connection is not None:
            await connection.close()
        if reader is not None and reader is not asyncio.current_task():
            reader.cancel()
            await asyncio.gather(reader, return_exceptions=True)

    async def reconnect(self) -> Snapshot:
        session = self._session
        if session is None:
            raise ClientStateError("client has no session")
        await self.close()
        await self.connect()
        return await self.resume(session)

    async def create_room(self, name: str) -> Snapshot:
        return await self._request("room.create", {"name": _input_text(name, "name")})

    async def join_room(self, game_id: str, name: str) -> Snapshot:
        return await self._request(
            "room.join", {"gameId": _input_text(game_id, "game_id"), "name": _input_text(name, "name")}
        )

    async def resume(self, session: Session | None = None) -> Snapshot:
        selected = session or self._session
        if selected is None:
            raise ClientStateError("client has no session")
        if not isinstance(selected, Session):
            raise ValueError("session must be a Session")
        return await self._request("session.resume", selected.to_payload())

    async def start_game(self) -> Snapshot:
        return await self._request("game.start", None)

    async def begin_auction(self) -> Snapshot:
        return await self._request("turn.auction", None)

    async def place_bid(self, amount: int, payment: Money) -> Snapshot:
        if isinstance(amount, bool) or not isinstance(amount, int) or amount <= 0:
            raise ValueError("amount must be a positive integer")
        return await self._request("auction.bid", {"amount": amount, "payment": _money(payment).to_payload()})

    async def close_auction(self) -> Snapshot:
        return await self._request("auction.close", None)

    async def resolve_auction(self, buy: bool, payment: Money | None = None) -> Snapshot:
        if not isinstance(buy, bool):
            raise ValueError("buy must be a boolean")
        if buy and payment is None:
            raise ValueError("buying requires payment")
        if not buy and payment is not None:
            raise ValueError("declining does not use payment")
        payload: dict[str, Any] = {"buy": buy}
        if payment is not None:
            payload["payment"] = _money(payment).to_payload()
        return await self._request("auction.resolve", payload)

    async def begin_trade(self, target_id: str, animal: Animal, offer: Money) -> Snapshot:
        if not isinstance(target_id, str) or not target_id:
            raise ValueError("target_id must be non-empty text")
        if not isinstance(animal, Animal):
            raise ValueError("animal must be an Animal")
        return await self._request(
            "turn.trade", {"targetId": target_id, "animal": animal.value, "offer": _money(offer).to_payload()}
        )

    async def accept_trade(self) -> Snapshot:
        return await self._request("trade.accept", None)

    async def counter_trade(self, offer: Money) -> Snapshot:
        return await self._request("trade.counter", {"offer": _money(offer).to_payload()})

    async def reoffer_trade(self, offer: Money) -> Snapshot:
        return await self._request("trade.reoffer", {"offer": _money(offer).to_payload()})

    async def snapshots(self) -> AsyncIterator[Snapshot]:
        queue: asyncio.Queue[Snapshot | KuhhandelError] = asyncio.Queue(maxsize=32)
        self._subscribers.add(queue)
        if self._snapshot is not None:
            queue.put_nowait(self._snapshot)
        try:
            while True:
                update = await queue.get()
                if isinstance(update, KuhhandelError):
                    raise update
                yield update
        finally:
            self._subscribers.discard(queue)

    async def _request(self, message_type: str, payload: Any) -> Snapshot:
        connection = self._connection
        if connection is None or not self.connected:
            raise ClientStateError("client is not connected")
        request_id = uuid4().hex
        encoded = json.dumps({"id": request_id, "type": message_type, "payload": payload}, separators=(",", ":"))
        if len(encoded.encode()) > 8 << 10:
            raise ValueError("request exceeds the server limit")
        future: asyncio.Future[Snapshot] = asyncio.get_running_loop().create_future()
        self._pending[request_id] = future
        try:
            async with self._send_lock:
                if self._connection is not connection:
                    raise ConnectionLost("connection changed before request was sent")
                await connection.send(encoded)
            return await asyncio.wait_for(asyncio.shield(future), self._timeout)
        except asyncio.TimeoutError as cause:
            timeout_error = RequestTimeout("request timed out; reconnect before sending another command")
            self._pending.pop(request_id, None)
            await self._drop(connection, timeout_error)
            raise timeout_error from cause
        except asyncio.CancelledError:
            self._pending.pop(request_id, None)
            await asyncio.shield(self._drop(connection, ConnectionLost("request was cancelled")))
            raise
        except KuhhandelError:
            self._pending.pop(request_id, None)
            raise
        except Exception as cause:
            send_error = ConnectionLost(f"could not send request: {cause}")
            self._pending.pop(request_id, None)
            await self._drop(connection, send_error)
            raise send_error from cause

    async def _read(self, connection: ClientConnection) -> None:
        failure: KuhhandelError = ConnectionLost("connection closed")
        try:
            while True:
                raw = await connection.recv()
                if not isinstance(raw, str):
                    raise ProtocolError("server sent a binary message")
                self._receive(_parse_message(raw))
        except asyncio.CancelledError:
            return
        except ProtocolError as error:
            failure = error
        except Exception as error:
            failure = ConnectionLost(f"connection lost: {error}")
        finally:
            if self._connection is connection:
                self._connection = None
                self._reader = None
                self._fail_pending(failure)
                self._publish_error(failure)

    def _receive(self, message: _SnapshotMessage | _ErrorMessage) -> None:
        if isinstance(message, _ErrorMessage):
            if message.request_id is None:
                raise ProtocolError(f"server error without request id: {message.error}")
            future = self._pending.pop(message.request_id, None)
            if future is not None and not future.done():
                future.set_exception(message.error)
            return
        identity = message.session or self._session
        if identity is not None and (
            identity.game_id != message.snapshot.public.game_id or identity.player_id != message.snapshot.self.player_id
        ):
            raise ProtocolError("snapshot does not match the session")
        if message.session is not None:
            self._session = message.session
        current = self._snapshot
        if (
            current is not None
            and current.public.game_id == message.snapshot.public.game_id
            and message.snapshot.version < current.version
        ):
            raise ProtocolError("snapshot version moved backwards")
        self._snapshot = message.snapshot
        self._publish_snapshot(message.snapshot)
        if message.request_id is not None:
            future = self._pending.pop(message.request_id, None)
            if future is not None and not future.done():
                future.set_result(message.snapshot)

    async def _drop(self, connection: ClientConnection, error: KuhhandelError) -> None:
        if self._connection is not connection:
            return
        reader = self._reader
        self._connection = None
        self._reader = None
        self._fail_pending(error)
        self._publish_error(error)
        await connection.close()
        if reader is not None and reader is not asyncio.current_task():
            reader.cancel()
            await asyncio.gather(reader, return_exceptions=True)

    def _fail_pending(self, error: KuhhandelError) -> None:
        pending, self._pending = self._pending, {}
        for future in pending.values():
            if not future.done():
                future.set_exception(error)

    def _publish_snapshot(self, snapshot: Snapshot) -> None:
        for subscriber in self._subscribers:
            _replace_if_full(subscriber, snapshot)

    def _publish_error(self, error: KuhhandelError) -> None:
        for subscriber in self._subscribers:
            _replace_if_full(subscriber, error)


def _parse_message(raw: str) -> _SnapshotMessage | _ErrorMessage:
    try:
        data = _object(json.loads(raw, object_pairs_hook=_unique_object), "response")
    except (json.JSONDecodeError, UnicodeDecodeError) as error:
        raise ProtocolError("server sent invalid JSON") from error
    message_type = _text(data.get("type"), "response.type")
    if message_type == "snapshot":
        _fields(data, ("type", "game"), ("requestId", "session"), "response")
        request_id = _optional_text(data.get("requestId"), "response.requestId")
        session = Session.from_payload(data["session"]) if "session" in data else None
        return _SnapshotMessage(request_id, session, Snapshot.from_payload(data["game"]))
    if message_type == "error":
        _fields(data, ("type", "error"), ("requestId",), "response")
        request_id = _optional_text(data.get("requestId"), "response.requestId")
        problem = _object(data["error"], "response.error")
        _fields(problem, ("code", "message"), (), "response.error")
        code = _text(problem["code"], "response.error.code")
        message = _text(problem["message"], "response.error.message")
        return _ErrorMessage(request_id, ServerError(code, message, request_id or ""))
    raise ProtocolError("server sent an unknown response type")


def _money(value: Money) -> Money:
    if not isinstance(value, Money):
        raise ValueError("payment must be Money")
    return value


def _input_text(value: str, name: str) -> str:
    if not isinstance(value, str) or not value:
        raise ValueError(f"{name} must be non-empty text")
    return value


def _unique_object(pairs: list[tuple[str, Any]]) -> dict[str, Any]:
    value: dict[str, Any] = {}
    for key, item in pairs:
        if key in value:
            raise ProtocolError("server sent duplicate JSON fields")
        value[key] = item
    return value


def _replace_if_full(queue: asyncio.Queue[Snapshot | KuhhandelError], value: Snapshot | KuhhandelError) -> None:
    if queue.full():
        queue.get_nowait()
    queue.put_nowait(value)
