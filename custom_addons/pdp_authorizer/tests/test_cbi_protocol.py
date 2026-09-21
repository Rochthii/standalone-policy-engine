import base64
import hashlib
import importlib.util
import pathlib
import unittest
from decimal import Decimal


MODULE_PATH = pathlib.Path(__file__).parents[1] / "cbi_protocol.py"
SPEC = importlib.util.spec_from_file_location("cbi_protocol", MODULE_PATH)
CBI = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(CBI)


def valid_intent():
    line_digest = hashlib.sha256(b"canonical purchase order lines").hexdigest()
    command_id = base64.urlsafe_b64encode(hashlib.sha256(b"command-42").digest())
    values = {
        "intent_version": "cbi.v1",
        "tenant_id": "tenant-a",
        "company_id": 7,
        "resource_type": "purchase.order",
        "resource_id": 42,
        "action": "action:CONFIRM_PURCHASE_ORDER",
        "vendor_id": 19,
        "currency_code": "USD",
        "currency_scale": 2,
        "amount_minor": 123456,
        "line_digest": line_digest,
        "record_state": "to approve",
        "record_write_version": "2026-09-20T10:11:12.123456Z",
        "creator_subject": "user:alice",
        "delegation_grant_id": 81,
        "delegator_subject": "user:bob",
        "agent_subject": "agent:procurement_copilot",
        "command_id": command_id.rstrip(b"=").decode("ascii"),
        "proof_version": "v2",
    }
    return CBI.with_state_witness(values)


class CanonicalBusinessIntentTest(unittest.TestCase):
    def test_exact_money_to_minor_units(self):
        self.assertEqual(CBI.amount_to_minor_units(Decimal("1234.56"), 2), 123456)
        self.assertEqual(CBI.amount_to_minor_units("5000", 0), 5000)

    def test_transport_context_and_major_amount_are_canonical(self):
        intent = CBI.with_state_witness(valid_intent())
        context = CBI.canonical_business_intent_context(intent)
        self.assertEqual(context["cbi.amount_minor"], "123456")
        self.assertEqual(context["cbi.proof_version"], "v2")
        self.assertEqual(CBI.minor_units_to_decimal(123456, 2), "1234.56")
        self.assertEqual(CBI.minor_units_to_decimal(120000, 2), "1200")
        self.assertEqual(CBI.minor_units_to_decimal(5, 3), "0.005")
        self.assertEqual(CBI.amount_to_minor_units("12.345", 3), 12345)

    def test_money_rejects_float_exponent_precision_and_range(self):
        invalid = (
            (12.34, 2),
            ("1e2", 2),
            (Decimal("1E+2"), 2),
            ("1.234", 2),
            ("-1", 2),
            ("9223372036854775808", 0),
            ("1", 7),
        )
        for amount, scale in invalid:
            with self.subTest(amount=amount, scale=scale):
                with self.assertRaises(ValueError):
                    CBI.amount_to_minor_units(amount, scale)

    def test_valid_fixture_is_deterministic(self):
        intent = valid_intent()
        first = CBI.canonical_business_intent_bytes(intent)
        second = CBI.canonical_business_intent_bytes(intent)
        self.assertEqual(first, second)
        self.assertEqual(
            CBI.canonical_business_intent_hash(intent),
            "e9b71c9a22c05e85cbbe0e6c3a767acb99631fd08d3306ed9f01c6ea577ff28c",
        )
        self.assertEqual(
            base64.b64encode(first).decode("ascii"),
            "UERQLUNBTk9OSUNBTC1CVVNJTkVTUy1JTlRFTlQAAAAGY2JpLnYxAAAACHRlbmFudC1hAAAAAAAAAAcAAAAOcHVyY2hhc2Uub3JkZXIAAAAAAAAAKgAAAB1hY3Rpb246Q09ORklSTV9QVVJDSEFTRV9PUkRFUgAAAAAAAAATAAAAA1VTRAAAAAAAAAACAAAAAAAB4kC89jzjrwo0cuYLHfRWVC9Gj1XoqkK1JP+TkHhxY1WQEgAAAAp0byBhcHByb3ZlAAAAGzIwMjYtMDktMjBUMTA6MTE6MTIuMTIzNDU2WolGC+uE0RTElP3xf75yvzJyzWGq0pa0x8dnbLezC9SqAAAACnVzZXI6YWxpY2UAAAAAAAAAUQAAAAh1c2VyOmJvYgAAABlhZ2VudDpwcm9jdXJlbWVudF9jb3BpbG90AAAAK01WV3pqYnlkQnhRVXZweC1RRlQ1ZEhJU0swdDk1Q3pudlRHUEVFcW5SRGsAAAACdjI=",
        )

    def test_rejects_malformed_fields(self):
        mutations = {
            "missing field": lambda value: value.pop("tenant_id"),
            "unknown field": lambda value: value.update(extra="unsafe"),
            "tenant whitespace": lambda value: value.update(tenant_id=" tenant-a"),
            "float integer": lambda value: value.update(amount_minor=123456.0),
            "company range": lambda value: value.update(company_id=0),
            "currency": lambda value: value.update(currency_code="usd"),
            "scale": lambda value: value.update(currency_scale=7),
            "digest": lambda value: value.update(line_digest="A" * 64),
            "timestamp": lambda value: value.update(record_write_version="2026-09-20T10:11:12Z"),
            "creator": lambda value: value.update(creator_subject="agent:alice"),
            "agent": lambda value: value.update(agent_subject="user:bot"),
            "command": lambda value: value.update(command_id=value["command_id"] + "="),
            "downgrade": lambda value: value.update(proof_version="v1"),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = valid_intent()
                mutate(changed)
                with self.assertRaises(ValueError):
                    CBI.canonical_business_intent_bytes(changed)

    def test_rejects_stale_state_witness(self):
        intent = valid_intent()
        intent["amount_minor"] += 1
        with self.assertRaisesRegex(ValueError, "state_witness"):
            CBI.canonical_business_intent_bytes(intent)


if __name__ == "__main__":
    unittest.main()
