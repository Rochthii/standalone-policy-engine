import base64
import hashlib
import hmac
import importlib.util
import json
import pathlib
import unittest


MODULE_PATH = pathlib.Path(__file__).parents[1] / "pdp_protocol.py"
SPEC = importlib.util.spec_from_file_location("pdp_protocol", MODULE_PATH)
PROTOCOL = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PROTOCOL)


TUPLE = {
    "tenant_id": "tenant-a",
    "grant_id": "grant-42",
    "delegator": "user:bob",
    "agent": "agent:procurement_copilot",
    "action": "action:APPROVE_PURCHASE_ORDER",
    "resource": "purchase_order:17",
    "amount": "2001",
    "delegation_chain": "user:bob,agent:procurement_copilot",
    "creator_id": "user:alice",
    "tool_context": "tool:auto_confirm_po",
    "execution_mode": "autonomous_run",
    "nonce": "nonce-0001",
    "issued_at": 1800000000,
    "valid_until": 1800003600,
}
KEY_ID = "key-2026-09"
SECRET = "test-delegation-secret-at-least-32-characters"


class ProtocolCompatibilityTest(unittest.TestCase):
    def test_proof_matches_go_golden_vector(self):
        self.assertEqual(
            PROTOCOL.sign_delegation_tuple(TUPLE, KEY_ID, SECRET),
            "v1.key-2026-09.c16450b09b9988d55e77d57915b055358e21e057e45ffe2d5474ac940a92faf3",
        )

    def test_fingerprint_changes_when_business_tuple_changes(self):
        original = PROTOCOL.delegation_fingerprint(TUPLE, KEY_ID)
        changed = dict(TUPLE, amount="2002")
        self.assertNotEqual(original, PROTOCOL.delegation_fingerprint(changed, KEY_ID))

    def test_jwt_is_short_lived_and_hs256_signed(self):
        token = PROTOCOL.issue_hs256_jwt(
            "agent:test", "tenant-a", SECRET, "issuer", "audience", now=1000
        )
        header, payload, signature = token.split(".")
        decode = lambda value: json.loads(
            base64.urlsafe_b64decode(value + "=" * (-len(value) % 4))
        )
        self.assertEqual(decode(header)["alg"], "HS256")
        self.assertEqual(decode(payload)["exp"], 1060)
        expected = hmac.new(
            SECRET.encode(), (header + "." + payload).encode(), hashlib.sha256
        ).digest()
        actual = base64.urlsafe_b64decode(signature + "=" * (-len(signature) % 4))
        self.assertTrue(hmac.compare_digest(actual, expected))

    def test_keyring_has_no_implicit_fallback(self):
        with self.assertRaises(PROTOCOL.PDPConfigurationError):
            PROTOCOL.load_delegation_keyring({})


if __name__ == "__main__":
    unittest.main()
