import asyncio
import json
import unittest
from typing import Any
from unittest.mock import AsyncMock, patch

from kuhhandel import (
    Animal,
    Client,
    ClientStateError,
    ConnectionLost,
    Money,
    ProtocolError,
    RequestTimeout,
    ServerError,
)

from .support import response


class FakeConnection:
    def __init__(self) -> None:
        self.incoming: asyncio.Queue[Any] = asyncio.Queue()
        self.sent: asyncio.Queue[str] = asyncio.Queue()
        self.closed = False

    async def send(self, message: str) -> None:
        if self.closed:
            raise ConnectionError("closed")
        await self.sent.put(message)

    async def recv(self) -> Any:
        value = await self.incoming.get()
        if isinstance(value, Exception):
            raise value
        return value

    async def close(self) -> None:
        self.closed = True

    async def next_request(self) -> Any:
        return json.loads(await asyncio.wait_for(self.sent.get(), 1))

    async def reply(self, value: Any) -> None:
        await self.incoming.put(json.dumps(value, separators=(",", ":")))


class ClientTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self) -> None:
        self.socket = FakeConnection()
        self.connector = patch("kuhhandel.client.connect", new=AsyncMock(return_value=self.socket))
        self.connector.start()
        self.client = Client("ws://example.test/ws", timeout=0.2)
        await self.client.connect()

    async def asyncTearDown(self) -> None:
        await self.client.close()
        self.connector.stop()

    async def complete(self, task: asyncio.Task, version: int = 1, session: bool = False) -> Any:
        request = await self.socket.next_request()
        await self.socket.reply(response(request["id"], version, session))
        return request, await task

    async def test_create_room_tracks_session_and_snapshot(self) -> None:
        task = asyncio.create_task(self.client.create_room("Alice"))
        request, result = await self.complete(task, session=True)
        self.assertEqual(request["type"], "room.create")
        self.assertEqual(request["payload"], {"name": "Alice"})
        self.assertEqual(result, self.client.snapshot)
        self.assertEqual(self.client.session.token, "session_secret")

    async def test_commands_send_exact_protocol_payloads(self) -> None:
        commands = [
            (self.client.start_game(), "game.start", None),
            (self.client.begin_auction(), "turn.auction", None),
            (
                self.client.place_bid(60, Money(ten=1, fifty=1)),
                "auction.bid",
                {"amount": 60, "payment": Money(ten=1, fifty=1).to_payload()},
            ),
            (self.client.close_auction(), "auction.close", None),
            (self.client.resolve_auction(False), "auction.resolve", {"buy": False}),
            (
                self.client.resolve_auction(True, Money(hundred=1)),
                "auction.resolve",
                {"buy": True, "payment": Money(hundred=1).to_payload()},
            ),
            (
                self.client.begin_trade("player_two", Animal.PIG, Money(zero=1)),
                "turn.trade",
                {"targetId": "player_two", "animal": "pig", "offer": Money(zero=1).to_payload()},
            ),
            (self.client.accept_trade(), "trade.accept", None),
            (self.client.counter_trade(Money(ten=1)), "trade.counter", {"offer": Money(ten=1).to_payload()}),
            (self.client.reoffer_trade(Money(fifty=1)), "trade.reoffer", {"offer": Money(fifty=1).to_payload()}),
        ]
        for call, message_type, payload in commands:
            with self.subTest(message_type=message_type):
                task = asyncio.create_task(call)
                request, result = await self.complete(task)
                self.assertEqual(request["type"], message_type)
                self.assertEqual(request["payload"], payload)
                self.assertEqual(result.version, 1)

    async def test_matches_concurrent_errors_and_replies_by_request_id(self) -> None:
        first = asyncio.create_task(self.client.begin_auction())
        second = asyncio.create_task(self.client.close_auction())
        first_request = await self.socket.next_request()
        second_request = await self.socket.next_request()
        await self.socket.reply(
            {
                "type": "error",
                "requestId": second_request["id"],
                "error": {"code": "forbidden", "message": "not allowed"},
            }
        )
        await self.socket.reply(response(first_request["id"]))
        self.assertEqual((await first).version, 1)
        with self.assertRaises(ServerError) as raised:
            await second
        self.assertEqual(raised.exception.code, "forbidden")
        self.assertEqual(raised.exception.request_id, second_request["id"])

    async def test_snapshot_stream_gets_current_and_broadcast_state(self) -> None:
        created = asyncio.create_task(self.client.create_room("Alice"))
        await self.complete(created, session=True)
        updates = self.client.snapshots()
        self.assertEqual((await updates.__anext__()).version, 1)
        await self.socket.reply(response(None, 2))
        self.assertEqual((await asyncio.wait_for(updates.__anext__(), 1)).version, 2)
        await updates.aclose()

    async def test_server_error_does_not_change_latest_snapshot(self) -> None:
        created = asyncio.create_task(self.client.create_room("Alice"))
        await self.complete(created, session=True)
        command = asyncio.create_task(self.client.begin_auction())
        request = await self.socket.next_request()
        await self.socket.reply(
            {
                "type": "error",
                "requestId": request["id"],
                "error": {"code": "not_your_turn", "message": "player is not active"},
            }
        )
        with self.assertRaises(ServerError):
            await command
        self.assertEqual(self.client.snapshot.version, 1)
        self.assertTrue(self.client.connected)

    async def test_malformed_server_message_fails_pending_requests(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        await self.socket.next_request()
        malformed = response(None)
        malformed["unexpected"] = True
        await self.socket.reply(malformed)
        with self.assertRaises(ProtocolError):
            await command
        self.assertFalse(self.client.connected)

    async def test_duplicate_server_fields_fail_pending_requests(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        request = await self.socket.next_request()
        await self.socket.incoming.put(
            f'{{"type":"error","requestId":"{request["id"]}","error":{{"code":"one","code":"two","message":"bad"}}}}'
        )
        with self.assertRaises(ProtocolError):
            await command
        self.assertFalse(self.client.connected)

    async def test_binary_server_message_fails_pending_requests(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        await self.socket.next_request()
        await self.socket.incoming.put(b"{}")
        with self.assertRaises(ProtocolError):
            await command
        self.assertFalse(self.client.connected)

    async def test_unsolicited_server_error_fails_pending_requests(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        await self.socket.next_request()
        await self.socket.reply({"type": "error", "error": {"code": "internal", "message": "failed"}})
        with self.assertRaises(ProtocolError):
            await command
        self.assertFalse(self.client.connected)

    async def test_snapshot_cannot_move_backwards(self) -> None:
        created = asyncio.create_task(self.client.create_room("Alice"))
        await self.complete(created, version=2, session=True)
        command = asyncio.create_task(self.client.begin_auction())
        request = await self.socket.next_request()
        await self.socket.reply(response(request["id"], version=1))
        with self.assertRaises(ProtocolError):
            await command
        self.assertFalse(self.client.connected)

    async def test_timeout_closes_connection_before_more_commands(self) -> None:
        socket = FakeConnection()
        with patch("kuhhandel.client.connect", new=AsyncMock(return_value=socket)):
            client = Client("ws://example.test/ws", timeout=0.01)
            await client.connect()
        with self.assertRaises(RequestTimeout):
            await client.create_room("Alice")
        self.assertFalse(client.connected)
        self.assertTrue(socket.closed)
        with self.assertRaises(ClientStateError):
            await client.start_game()

    async def test_send_failure_closes_connection_and_uses_sdk_error(self) -> None:
        self.socket.closed = True
        with self.assertRaises(ConnectionLost):
            await self.client.create_room("Alice")
        self.assertFalse(self.client.connected)

    async def test_connect_failure_uses_sdk_error(self) -> None:
        client = Client("ws://example.test/ws")
        with patch("kuhhandel.client.connect", new=AsyncMock(side_effect=OSError("refused"))):
            with self.assertRaises(ConnectionLost):
                await client.connect()
        self.assertFalse(client.connected)

    async def test_cancelled_request_closes_connection(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        await self.socket.next_request()
        command.cancel()
        with self.assertRaises(asyncio.CancelledError):
            await command
        self.assertFalse(self.client.connected)
        self.assertTrue(self.socket.closed)

    async def test_reconnect_resumes_saved_session(self) -> None:
        created = asyncio.create_task(self.client.create_room("Alice"))
        await self.complete(created, session=True)
        replacement = FakeConnection()
        with patch("kuhhandel.client.connect", new=AsyncMock(return_value=replacement)):
            reconnecting = asyncio.create_task(self.client.reconnect())
            request = json.loads(await asyncio.wait_for(replacement.sent.get(), 1))
            self.assertEqual(request["type"], "session.resume")
            self.assertEqual(request["payload"]["token"], "session_secret")
            await replacement.reply(response(request["id"], session=True))
            self.assertEqual((await reconnecting).version, 1)
        self.socket = replacement

    async def test_close_fails_pending_request(self) -> None:
        command = asyncio.create_task(self.client.create_room("Alice"))
        await self.socket.next_request()
        await self.client.close()
        with self.assertRaises(ConnectionLost):
            await command

    async def test_local_argument_checks_do_not_send(self) -> None:
        calls = [
            self.client.create_room(""),
            self.client.join_room("", "Bob"),
            self.client.place_bid(0, Money()),
            self.client.resolve_auction(True),
            self.client.resolve_auction(False, Money()),
            self.client.begin_trade("player_two", "pig", Money()),
        ]
        for call in calls:
            with self.assertRaises(ValueError):
                await call
        self.assertTrue(self.socket.sent.empty())

    async def test_oversized_request_is_not_sent(self) -> None:
        with self.assertRaises(ValueError):
            await self.client.create_room("a" * 9000)
        self.assertTrue(self.client.connected)
        self.assertTrue(self.socket.sent.empty())


if __name__ == "__main__":
    unittest.main()
