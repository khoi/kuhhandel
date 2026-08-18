# Kuhhandel Python SDK

An async client for the authoritative Kuhhandel server. It exposes typed, immutable snapshots and one method for each server command. It does not predict game state.

## Install

```sh
python -m pip install ./sdk/python
```

Python 3.9 or newer is required.

## Use

```python
import asyncio

from kuhhandel import Client, Money, ServerError


async def main():
    async with Client("ws://localhost:8080/ws") as client:
        snapshot = await client.create_room("Alice")
        print(snapshot.public.game_id)
        print(client.session)

        try:
            snapshot = await client.start_game()
        except ServerError as error:
            print(error.code, error.message)

        async for snapshot in client.snapshots():
            print(snapshot.version, snapshot.public.phase, snapshot.self.legal_actions)


asyncio.run(main())
```

Join and resume:

```python
await client.join_room(game_id, "Bob")
saved_session = client.session

await client.close()
await client.connect()
await client.resume(saved_session)
```

Auction and trade commands:

```python
await client.begin_auction()
await client.place_bid(60, Money(ten=1, fifty=1))
await client.close_auction()
await client.resolve_auction(buy=False)

await client.begin_trade(target_id, animal, Money(zero=1))
await client.accept_trade()
await client.counter_trade(Money(fifty=1))
await client.reoffer_trade(Money(hundred=1))
```

Keep `Session.token` secret. Use `wss://` outside a trusted local network. A request timeout closes the connection because the command may have reached the server. Call `reconnect()` to resume from the server snapshot before sending another command.

## Test

```sh
cd sdk/python
python -m unittest discover -v
```

The suite includes model, protocol, concurrency, failure, reconnect, and live Go server tests.
