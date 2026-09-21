import base64
import copy
import json
import pathlib
import sys
import unittest


ROOT = pathlib.Path(__file__).parents[3]
ADDON = ROOT / "custom_addons" / "pdp_authorizer"
sys.path.insert(0, str(ADDON))

import cbi_lines as LINES  # noqa: E402
import cbi_protocol as CBI  # noqa: E402
import delegation_proof_v2 as PROOF  # noqa: E402


def load_fixture():
    path = ROOT / "internal" / "security" / "testdata" / "cbi_v1_golden.json"
    return json.loads(path.read_text(encoding="utf-8"))


def build_intent(fixture):
    intent = dict(fixture["intent"])
    intent["amount_minor"] = CBI.amount_to_minor_units(
        fixture["amount_total"], intent["currency_scale"]
    )
    intent["line_digest"] = LINES.canonical_purchase_order_line_digest(fixture["lines"])
    intent.pop("state_witness", None)
    return CBI.with_state_witness(intent)


class CBIV2GoldenVectorTest(unittest.TestCase):
    def test_cross_language_line_intent_and_proof_vector(self):
        fixture = load_fixture()
        expected = fixture["expected"]
        line_bytes = LINES.canonical_purchase_order_lines_bytes(fixture["lines"])
        self.assertEqual(base64.b64encode(line_bytes).decode("ascii"), expected["line_bytes_base64"])
        intent = build_intent(fixture)
        self.assertEqual(intent["line_digest"], expected["line_digest"])
        self.assertEqual(intent["state_witness"], expected["state_witness"])
        intent_bytes = CBI.canonical_business_intent_bytes(intent)
        self.assertEqual(base64.b64encode(intent_bytes).decode("ascii"), expected["intent_bytes_base64"])
        self.assertEqual(CBI.canonical_business_intent_hash(intent), expected["intent_hash"])
        self.assertEqual(
            PROOF.sign_delegation_proof_v2(
                intent,
                fixture["key_id"],
                fixture["secret"],
                fixture["issued_at"],
                fixture["valid_until"],
            ),
            expected["proof"],
        )

    def test_input_slice_order_is_not_semantic(self):
        fixture = load_fixture()
        original = LINES.canonical_purchase_order_lines_bytes(fixture["lines"])
        reversed_input = list(reversed(fixture["lines"]))
        self.assertEqual(original, LINES.canonical_purchase_order_lines_bytes(reversed_input))

    def test_material_tamper_changes_hash_or_fails_validation(self):
        fixture = load_fixture()
        original = build_intent(fixture)
        original_hash = CBI.canonical_business_intent_hash(original)
        mutations = {
            "amount": lambda value: value.update(amount_minor=value["amount_minor"] + 1),
            "currency": lambda value: value.update(currency_code="EUR"),
            "vendor": lambda value: value.update(vendor_id=value["vendor_id"] + 1),
            "resource": lambda value: value.update(resource_id=value["resource_id"] + 1),
            "state": lambda value: value.update(record_state="draft"),
            "record version": lambda value: value.update(
                record_write_version="2026-09-20T10:11:12.123457Z"
            ),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = dict(original)
                mutate(changed)
                changed = CBI.with_state_witness(changed)
                self.assertNotEqual(CBI.canonical_business_intent_hash(changed), original_hash)
        invalid_action = dict(original, action="action:CANCEL")
        with self.assertRaises(ValueError):
            CBI.canonical_business_intent_bytes(invalid_action)

    def test_line_tamper_changes_digest(self):
        fixture = load_fixture()
        original = LINES.canonical_purchase_order_line_digest(fixture["lines"])
        mutations = {
            "add": lambda lines: lines.append(
                dict(lines[0], line_id=103, sequence=20)
            ),
            "remove": lambda lines: lines.pop(),
            "sequence": lambda lines: lines[0].update(sequence=lines[0]["sequence"] + 1),
            "product": lambda lines: lines[0].update(product_id=lines[0]["product_id"] + 1),
            "description": lambda lines: lines[0].update(description=lines[0]["description"] + " changed"),
            "uom": lambda lines: lines[0].update(uom_id=lines[0]["uom_id"] + 1),
            "quantity": lambda lines: lines[0].update(quantity="3"),
            "unit price": lambda lines: lines[0].update(unit_price="123.457"),
            "tax": lambda lines: lines[0].update(tax_ids=[3]),
            "planned": lambda lines: lines[0].update(planned_at="2026-09-25T08:30:00.000001Z"),
            "line version": lambda lines: lines[0].update(
                line_write_version="2026-09-20T10:10:00.000002Z"
            ),
        }
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(fixture["lines"])
                mutate(changed)
                self.assertNotEqual(LINES.canonical_purchase_order_line_digest(changed), original)


if __name__ == "__main__":
    unittest.main()
