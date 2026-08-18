import asyncio
import socket
import subprocess
import tempfile
import unittest
from pathlib import Path

from kuhhandel import Client, Phase, ServerError, Status

REPOSITORY = Path(__file__).resolve().parents[3]


class GoServerTests(unittest.IsolatedAsyncioTestCase):
    @classmethod
    def setUpClass(cls) -> None:
        cls.build_directory = tempfile.TemporaryDirectory()
        cls.binary = Path(cls.build_directory.name) / "kuhhandel"
        subprocess.run(["go", "build", "-o", str(cls.binary), "./cmd/kuhhandel"], cwd=REPOSITORY, check=True)

    @classmethod
    def tearDownClass(cls) -> None:
        cls.build_directory.cleanup()

    async def asyncSetUp(self) -> None:
        self.data_directory = tempfile.TemporaryDirectory()
        with socket.socket() as listener:
            listener.bind(("127.0.0.1", 0))
            self.port = listener.getsockname()[1]
        self.process = await asyncio.create_subprocess_exec(
            str(self.binary),
            "-addr",
            f"127.0.0.1:{self.port}",
            "-db",
            str(Path(self.data_directory.name) / "games.db"),
            cwd=REPOSITORY,
            stdout=subprocess.DEVNULL,
            stderr=asyncio.subprocess.PIPE,
        )
        self.clients = []
        for _ in range(100):
            if self.process.returncode is not None:
                self.fail((await self.process.stderr.read()).decode())
            try:
                _, writer = await asyncio.open_connection("127.0.0.1", self.port)
                writer.close()
                await writer.wait_closed()
                break
            except OSError:
                await asyncio.sleep(0.01)
        else:
            self.fail("server did not start")

    async def asyncTearDown(self) -> None:
        await asyncio.gather(*(client.close() for client in self.clients), return_exceptions=True)
        self.process.terminate()
        try:
            await asyncio.wait_for(self.process.wait(), 5)
        except asyncio.TimeoutError:
            self.process.kill()
            await self.process.wait()
        self.data_directory.cleanup()

    async def new_client(self) -> Client:
        client = Client(f"ws://127.0.0.1:{self.port}/ws")
        await client.connect()
        self.clients.append(client)
        return client

    async def wait_for_version(self, client: Client, version: int) -> None:
        for _ in range(100):
            if client.snapshot is not None and client.snapshot.version >= version:
                return
            await asyncio.sleep(0.01)
        self.fail(f"client did not reach version {version}")

    async def test_room_play_errors_broadcasts_and_session_replacement(self) -> None:
        host, bob, carol = await asyncio.gather(self.new_client(), self.new_client(), self.new_client())
        created = await host.create_room("Alice")
        game_id = created.public.game_id
        await bob.join_room(game_id, "Bob")
        joined = await carol.join_room(game_id, "Carol")
        await self.wait_for_version(host, joined.version)
        self.assertEqual(len(host.snapshot.public.players), 3)
        self.assertEqual(host.snapshot.public.status, Status.LOBBY)
        with self.assertRaises(ServerError) as forbidden:
            await bob.start_game()
        self.assertEqual(forbidden.exception.code, "forbidden")
        started = await host.start_game()
        await self.wait_for_version(bob, started.version)
        self.assertEqual(started.public.status, Status.PLAYING)
        self.assertEqual(started.public.phase, Phase.TURN)
        self.assertEqual(started.self.money.total, 90)
        self.assertEqual(bob.snapshot.self.player_id, bob.session.player_id)
        with self.assertRaises(ServerError) as wrong_turn:
            await bob.begin_auction()
        self.assertEqual(wrong_turn.exception.code, "not_your_turn")
        replacement = await self.new_client()
        resumed = await replacement.resume(host.session)
        self.assertEqual(resumed.version, started.version)
        for _ in range(100):
            if not host.connected:
                break
            await asyncio.sleep(0.01)
        self.assertFalse(host.connected)
        auction = await replacement.begin_auction()
        self.assertEqual(auction.public.phase, Phase.AUCTION)
        self.assertEqual(auction.public.auction.auctioneer_id, host.session.player_id)
        await self.wait_for_version(carol, auction.version)
        self.assertIsNone(carol.snapshot.self.bid_payment)


if __name__ == "__main__":
    unittest.main()
