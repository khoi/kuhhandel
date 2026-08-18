import unittest
from dataclasses import FrozenInstanceError

from kuhhandel import Animal, LegalAction, Money, Phase, ProtocolError, Session, Snapshot, Status

from .support import snapshot


class MoneyTests(unittest.TestCase):
    def test_money_round_trip_and_totals(self) -> None:
        value = Money(zero=2, ten=4, fifty=1, hundred=2, two_hundred=1, five_hundred=1)
        self.assertEqual(value.total, 990)
        self.assertEqual(value.card_count, 11)
        self.assertEqual(Money.from_payload(value.to_payload()), value)

    def test_money_rejects_invalid_counts(self) -> None:
        for count in (-1, True, 1.5):
            with self.subTest(count=count), self.assertRaises(ValueError):
                Money(ten=count)

    def test_money_payload_requires_every_denomination(self) -> None:
        with self.assertRaises(ProtocolError):
            Money.from_payload({"10": 1})


class SnapshotTests(unittest.TestCase):
    def test_session_repr_hides_token(self) -> None:
        session = Session("game_one", "player_one", "session_secret")
        self.assertNotIn(session.token, repr(session))

    def test_parses_typed_immutable_snapshot(self) -> None:
        raw = snapshot(7, phase="turn")
        raw["public"]["turnPlayerId"] = "player_one"
        raw["public"]["auction"] = {
            "animal": "cow",
            "auctioneerId": "player_one",
            "highestBid": 50,
            "highestBidderId": "player_two",
        }
        raw["self"]["legalActions"] = ["turn.auction", "turn.trade"]
        parsed = Snapshot.from_payload(raw)
        self.assertEqual(parsed.version, 7)
        self.assertEqual(parsed.public.status, Status.PLAYING)
        self.assertEqual(parsed.public.phase, Phase.TURN)
        self.assertEqual(parsed.public.auction.animal, Animal.COW)
        self.assertEqual(parsed.self.legal_actions, (LegalAction.BEGIN_AUCTION, LegalAction.BEGIN_TRADE))
        self.assertEqual(parsed.public.players[0].animals[Animal.PIG], 1)
        with self.assertRaises(TypeError):
            parsed.public.players[0].animals[Animal.COW] = 2
        with self.assertRaises(FrozenInstanceError):
            parsed.version = 8

    def test_rejects_unknown_fields(self) -> None:
        raw = snapshot()
        raw["public"]["secret"] = "leak"
        with self.assertRaises(ProtocolError):
            Snapshot.from_payload(raw)

    def test_rejects_invalid_enums_and_numbers(self) -> None:
        cases = []
        invalid_animal = snapshot()
        invalid_animal["public"]["players"][0]["animals"] = {"dragon": 1}
        cases.append(invalid_animal)
        invalid_phase = snapshot()
        invalid_phase["public"]["phase"] = "unknown"
        cases.append(invalid_phase)
        invalid_version = snapshot()
        invalid_version["version"] = True
        cases.append(invalid_version)
        for raw in cases:
            with self.subTest(raw=raw), self.assertRaises(ProtocolError):
                Snapshot.from_payload(raw)


if __name__ == "__main__":
    unittest.main()
