"""Real Odoo/PDP final execution of one approved delegated PO command."""

from datetime import timedelta
from decimal import Decimal
from unittest.mock import patch

import grpc
from psycopg2 import IntegrityError, errors, sql

from odoo import Command, fields
from odoo.api import call_kw
from odoo.exceptions import AccessError, UserError
from odoo.tests import TransactionCase, tagged

from ..cbi_protocol import amount_to_minor_units, with_state_witness
from ..models import pdp_client, purchase_order_final
from ..pdp_protocol import issue_jwt_from_environment


@tagged("post_install", "-at_install")
class TestApprovedFinalExecution(TransactionCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        import os

        manager = cls.env.ref("purchase.group_purchase_manager")
        cls.env.company.write(
            {
                "pdp_tenant_id": os.environ["PDP_TENANT_ID"],
                "po_double_validation": "one_step",
            }
        )
        cls.env.user.write({"pdp_department": "Procurement"})
        cls.creator = cls._user("TXN03 Creator", "txn03_creator", manager)
        cls.approver = cls._user(
            "TXN03 Approver", "pdp_e2e_approver", manager
        )
        cls.vendor = cls.env["res.partner"].create(
            {"name": "TXN03 Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "TXN03 Product", "purchase_ok": True}
        )
        cls.env.cr.execute(
            """
            SELECT setval(
                pg_get_serial_sequence('pdp_delegation_grant', 'id'),
                2050000000 +
                    (EXTRACT(EPOCH FROM clock_timestamp())::bigint % 10000000),
                true
            )
            """
        )

    @classmethod
    def _user(cls, name, login, manager):
        return cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": name,
                "login": login,
                "email": "%s@example.test" % login,
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "pdp_department": "Procurement",
                "groups_id": [Command.link(manager.id)],
            }
        )

    def setUp(self):
        super().setUp()
        now = fields.Datetime.now()
        self.grant = self.env["pdp.delegation.grant"].create(
            {
                "user_id": self.env.user.id,
                "agent_id": "agent:procurement_copilot",
                "max_amount": 2000,
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
            }
        )
        self.grant.action_activate()

    def _approved_order(self, issue=True, two_lines=False):
        lines = [
            Command.create(
                {
                    "name": self.product.display_name,
                    "product_id": self.product.id,
                    "product_qty": 1,
                    "product_uom": self.product.uom_po_id.id,
                    "price_unit": 1250 if two_lines else 2500,
                    "date_planned": fields.Datetime.now(),
                    "sequence": 10,
                }
            )
        ]
        if two_lines:
            lines.append(
                Command.create(
                    {
                        "name": "Second approved line",
                        "product_id": self.product.id,
                        "product_qty": 1,
                        "product_uom": self.product.uom_po_id.id,
                        "price_unit": 1250,
                        "date_planned": fields.Datetime.now(),
                        "sequence": 20,
                    }
                )
            )
        order = self.env["purchase.order"].with_user(self.creator).sudo().create(
            {
                "partner_id": self.vendor.id,
                "company_id": self.env.company.id,
                "pdp_department": "Procurement",
                "ai_agent_id": self.grant.agent_id,
                "delegated_by_id": self.grant.user_id.id,
                "delegation_grant_id": self.grant.id,
                "order_line": lines,
            }
        )
        self.assertTrue(order.button_confirm())
        approval = self.env["pdp.approval.request"].sudo().search(
            [("purchase_order_id", "=", order.id)]
        ).ensure_one()
        if issue:
            self.assertTrue(
                approval.with_user(self.approver).action_issue_capability()
            )
        return order, approval, approval.authorization_attempt_id

    def _public_confirm(self, order):
        model = self.env["purchase.order"].with_user(self.creator)
        return call_kw(model, "button_confirm", [[order.id]], {})

    def _assert_not_consumed(
        self, order, approval, attempt, approval_state, order_state="to approve"
    ):
        order.invalidate_recordset()
        approval.invalidate_recordset()
        attempt.invalidate_recordset()
        self.assertEqual(order.state, order_state)
        self.assertEqual(approval.state, approval_state)
        self.assertEqual(attempt.state, "approval_required")
        self.assertEqual(attempt.decision, "allow")
        self.assertNotEqual(attempt.result_state, "purchase")

    def test_public_confirm_consumes_approval_with_one_final_effect(self):
        order, approval, attempt = self._approved_order()

        self.assertTrue(self._public_confirm(order))

        order.invalidate_recordset()
        approval.invalidate_recordset()
        attempt.invalidate_recordset()
        self.assertEqual(order.state, "purchase")
        self.assertEqual(order.pdp_status, "allow")
        self.assertEqual(approval.state, "consumed")
        self.assertEqual(approval.terminal_reason, "executed")
        self.assertTrue(approval.terminal_at)
        self.assertEqual(attempt.state, "executed")
        self.assertEqual(attempt.decision, "allow")
        self.assertEqual(attempt.result_state, "purchase")
        self.assertTrue(attempt.completed_at)

    def test_pending_approval_cannot_reach_final_state(self):
        order, approval, attempt = self._approved_order(issue=False)

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "pending")

    def test_public_final_rejects_expired_and_inactive_grants(self):
        for values, reason in (
            ({"valid_until": fields.Datetime.now() - timedelta(seconds=1)}, "delegation_expired"),
            ({"state": "draft"}, "delegation_inactive"),
        ):
            with self.subTest(reason=reason):
                original = self.grant.read(["state", "valid_until"])[0]
                order, approval, attempt = self._approved_order()
                binding = (approval.intent_hash, approval.one_time_id, approval.command_id)
                self.grant.write(values)
                with self.env.cr.savepoint():
                    self.assertTrue(self._public_confirm(order))
                self._assert_not_consumed(order, approval, attempt, "invalidated")
                self.assertEqual(approval.terminal_reason, reason)
                self.assertEqual(binding, (approval.intent_hash, approval.one_time_id, approval.command_id))
                self.grant.write({key: original[key] for key in ("state", "valid_until")})

    def test_exact_one_minor_unit_persisted_change_rejects_approval(self):
        # Tax-free fixture isolates the exact currency quantum, not float rounding.
        self.product.supplier_taxes_id = [Command.clear()]
        order, approval, attempt = self._approved_order()
        self.assertFalse(order.order_line.taxes_id)
        scale = order.currency_id.decimal_places
        before = approval._stored_intent()["amount_minor"]
        quantum = Decimal(1).scaleb(-scale)
        order.order_line.write({"price_unit": float(Decimal("2500") + quantum)})
        self.env.flush_all()
        self.env.cr.execute("SELECT amount_total::text FROM purchase_order WHERE id = %s", [order.id])
        self.assertEqual(amount_to_minor_units(self.env.cr.fetchone()[0], scale), before + 1)
        with self.env.cr.savepoint():
            self.assertTrue(self._public_confirm(order))
        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_line_guard_does_not_touch_ordinary_parent(self):
        order = self.env["purchase.order"].create({
            "partner_id": self.vendor.id,
            "order_line": [Command.create({"name": "Ordinary line", "product_id": self.product.id,
                "product_qty": 1, "product_uom": self.product.uom_po_id.id,
                "price_unit": 100, "date_planned": fields.Datetime.now()})],
        })
        self.env.flush_all()
        self.env.cr.execute("SELECT write_date FROM purchase_order WHERE id = %s", [order.id])
        version = self.env.cr.fetchone()[0]
        order.order_line.write({"name": "Ordinary edited line"})
        self.env.flush_all()
        self.env.cr.execute("SELECT write_date FROM purchase_order WHERE id = %s", [order.id])
        self.assertEqual(self.env.cr.fetchone()[0], version)
        self.assertFalse(order.pdp_delegated_scope)
        self.assertEqual(order.order_line.name, "Ordinary edited line")

    def test_reissuing_approval_does_not_supersede_capability(self):
        order, approval, attempt = self._approved_order()
        original = (
            approval.one_time_id,
            approval.capability_payload_b64,
            approval.capability_signature_b64,
            approval.capability_fingerprint,
        )

        self.assertTrue(approval.with_user(self.approver).action_issue_capability())

        approval.invalidate_recordset()
        self.assertEqual(
            original,
            (
                approval.one_time_id,
                approval.capability_payload_b64,
                approval.capability_signature_b64,
                approval.capability_fingerprint,
            ),
        )
        self.assertTrue(self._public_confirm(order))
        order.invalidate_recordset()
        attempt.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "purchase")
        self.assertEqual(approval.state, "consumed")
        self.assertEqual(attempt.state, "executed")

    def test_approval_capability_cannot_be_reused_for_another_command(self):
        first_order, first, first_attempt = self._approved_order()
        second_order, second, second_attempt = self._approved_order()
        self.assertNotEqual(first.command_id, second.command_id)
        self.assertNotEqual(first.one_time_id, second.one_time_id)

        with self.assertRaises(IntegrityError):
            with self.env.cr.savepoint():
                second.write({"one_time_id": first.one_time_id})

        self._assert_not_consumed(first_order, first, first_attempt, "approved")
        self._assert_not_consumed(second_order, second, second_attempt, "approved")

        # Copying the signed bytes while keeping the recipient command's unique ID
        # must still fail signature/binding verification at the final ERP route.
        second.write(
            {
                "capability_payload_b64": first.capability_payload_b64,
                "capability_signature_b64": first.capability_signature_b64,
            }
        )
        self.assertTrue(self._public_confirm(second_order))

        self._assert_not_consumed(first_order, first, first_attempt, "approved")
        self._assert_not_consumed(
            second_order, second, second_attempt, "invalidated"
        )
        self.assertEqual(second.terminal_reason, "capability_verification_failed")

    def test_changed_intent_invalidates_without_consumption(self):
        order, approval, attempt = self._approved_order()
        order.order_line.write({"price_unit": 2600})

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_material_line_edits_never_reuse_approved_intent(self):
        purchase_tax = self.env["account.tax"].create(
            {"name": "EVAL01 Purchase Tax", "amount": 5.0, "type_tax_use": "purchase"}
        )
        alternate_uom = self.env["uom.uom"].create(
            {
                "name": "EVAL01 Pair",
                "category_id": self.product.uom_po_id.category_id.id,
                "uom_type": "bigger",
                "factor_inv": 2,
            }
        )
        edits = (
            ("quantity", {"product_qty": 2}),
            ("description", {"name": "changed approved line"}),
            ("planned date", {"date_planned": fields.Datetime.now() + timedelta(days=1)}),
            ("tax", {"taxes_id": [Command.set([purchase_tax.id])]}),
            ("uom", {"product_uom": alternate_uom.id}),
        )
        for name, values in edits:
            with self.subTest(edit=name):
                order, approval, attempt = self._approved_order()
                order.order_line.write(values)
                self.assertTrue(self._public_confirm(order))
                self._assert_not_consumed(order, approval, attempt, "invalidated")
                self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_persisted_line_version_change_invalidates_approved_intent(self):
        order, approval, attempt = self._approved_order()
        order.order_line.flush_recordset()
        self.env.cr.execute(
            "UPDATE purchase_order_line SET write_date = write_date + INTERVAL '1 second' "
            "WHERE order_id = %s",
            [order.id],
        )

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_line_shape_changes_never_reuse_approved_intent(self):
        other_product = self.env["product.product"].create(
            {"name": "EVAL01 replacement product", "purchase_ok": True}
        )
        for change in ("add", "remove", "sequence", "product"):
            with self.subTest(change=change):
                order, approval, attempt = self._approved_order()
                if change == "add":
                    order.write(
                        {
                            "order_line": [
                                Command.create(
                                    {
                                        "name": other_product.display_name,
                                        "product_id": other_product.id,
                                        "product_qty": 1,
                                        "product_uom": other_product.uom_po_id.id,
                                        "price_unit": 1,
                                        "date_planned": fields.Datetime.now(),
                                    }
                                )
                            ]
                        }
                    )
                elif change == "remove":
                    order.order_line.unlink()
                elif change == "sequence":
                    order.order_line.write({"sequence": 20})
                else:
                    order.order_line.write({"product_id": other_product.id})

                self.assertTrue(self._public_confirm(order))
                self._assert_not_consumed(order, approval, attempt, "invalidated")
                self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_two_line_reorder_never_reuses_approved_intent(self):
        order, approval, attempt = self._approved_order(two_lines=True)
        first, second = order.order_line.sorted(key=lambda line: (line.sequence, line.id))
        self.assertEqual((first.sequence, second.sequence), (10, 20))

        first.write({"sequence": 30})
        self.assertEqual(
            order.order_line.sorted(key=lambda line: (line.sequence, line.id)).ids,
            [second.id, first.id],
        )
        self.assertTrue(self._public_confirm(order))
        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_vendor_edit_never_reuses_approved_intent(self):
        order, approval, attempt = self._approved_order()
        vendor = self.env["res.partner"].create(
            {"name": "EVAL01 replacement vendor", "supplier_rank": 1}
        )
        order.write({"partner_id": vendor.id})

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(order.partner_id, vendor)
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_currency_edit_never_reuses_approved_intent(self):
        order, approval, attempt = self._approved_order()
        currency = self.env.ref(
            "base.EUR" if order.currency_id.name != "EUR" else "base.USD"
        )
        currency.active = True
        old_amount = order.amount_total
        old_binding = (approval.intent_hash, approval.one_time_id, approval.command_id)
        model = self.env["purchase.order"].with_user(self.creator)
        self.assertTrue(call_kw(model, "write", [[order.id], {"currency_id": currency.id}], {}))

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(order.currency_id, currency)
        self.assertEqual(order.amount_total, old_amount)
        self.assertEqual(approval.terminal_reason, "intent_changed")
        self.assertEqual(
            (approval.intent_hash, approval.one_time_id, approval.command_id), old_binding
        )

    def test_final_material_context_tamper_with_old_proof_fails_closed(self):
        target, target_approval, target_attempt = self._approved_order()
        company = self.env["res.company"].create(
            {"name": "CBI-N02 Other Company", "pdp_tenant_id": self.env.company.pdp_tenant_id}
        )
        vendor = self.env["res.partner"].create({"name": "CBI-N02 Other Vendor"})
        currency = "EUR" if target.currency_id.name != "EUR" else "USD"
        order_class = type(target)
        original = order_class._protected_delegated_request
        for field, replacement in (
            ("tenant_id", "00000000-0000-0000-0000-000000000099"),
            ("company_id", company.id),
            ("action", "action:APPROVE_PURCHASE_ORDER"),
            ("resource_id", target.id),
            ("vendor_id", vendor.id),
            ("currency_code", currency),
        ):
            with self.subTest(field=field):
                order, approval, attempt = self._approved_order()
                injected = []

                def tamper_request(record, intent, proof, issued_at, valid_until):
                    self.assertNotEqual(intent[field], replacement)
                    if field == "action":
                        # CBI v1 permits only confirm: inject the unsupported wire value.
                        request = original(record, intent, proof, issued_at, valid_until)
                        request["context"]["cbi.action"] = replacement
                    else:
                        altered = with_state_witness({**intent, field: replacement})
                        request = original(record, altered, proof, issued_at, valid_until)
                    self.assertEqual(request["context"]["delegation_proof"], proof)
                    injected.append(field)
                    return request

                # Fault injection changes the request, never the real PDP decision.
                with patch.object(order_class, "_protected_delegated_request", tamper_request):
                    with self.assertRaises(AccessError) as denied:
                        with self.env.cr.savepoint():
                            self._public_confirm(order)

                self.assertEqual(injected, [field])
                self.assertIsInstance(denied.exception.__cause__, grpc.RpcError)
                self.assertEqual(denied.exception.__cause__.code(), grpc.StatusCode.PERMISSION_DENIED)
                self._assert_not_consumed(order, approval, attempt, "approved")
                self._assert_not_consumed(target, target_approval, target_attempt, "approved")

    def test_post_approval_state_change_invalidates_old_capability(self):
        for state in ("draft", "sent", "cancel"):
            with self.subTest(state=state):
                order, approval, attempt = self._approved_order()
                old_binding = (approval.intent_hash, approval.one_time_id, approval.command_id)
                model = self.env["purchase.order"].with_user(self.creator)
                self.assertTrue(call_kw(model, "write", [[order.id], {"state": state}], {}))
                order.flush_recordset(["state"])
                order.invalidate_recordset(["state"])
                self.assertEqual(order.state, state)

                self.assertTrue(self._public_confirm(order))

                self._assert_not_consumed(order, approval, attempt, "invalidated", state)
                self.assertEqual(approval.terminal_reason, "intent_changed")
                self.assertEqual(
                    (approval.intent_hash, approval.one_time_id, approval.command_id),
                    old_binding,
                )
                with self.assertRaises(AccessError):
                    with self.env.cr.savepoint():
                        self._public_confirm(order)
                self._assert_not_consumed(order, approval, attempt, "invalidated", state)

    def test_explicit_parent_write_version_change_invalidates_approval(self):
        order, approval, attempt = self._approved_order()
        self.env.cr.execute(
            "UPDATE purchase_order SET write_date = write_date + INTERVAL '1 second' "
            "WHERE id = %s",
            [order.id],
        )

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "intent_changed")

    def test_final_agent_jwt_identity_and_tenant_mismatch_fail_closed(self):
        for label, token in (
            ("malformed", "not-a-jwt"),
            (
                "wrong agent",
                issue_jwt_from_environment(
                    "agent:other", self.env.company.pdp_tenant_id
                ),
            ),
            (
                "wrong tenant",
                issue_jwt_from_environment(
                    self.grant.agent_id, "00000000-0000-0000-0000-000000000099"
                ),
            ),
        ):
            with self.subTest(label=label):
                order, approval, attempt = self._approved_order()
                with patch.object(
                    purchase_order_final, "issue_jwt_from_environment", return_value=token
                ):
                    with self.assertRaises(AccessError):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                self._assert_not_consumed(order, approval, attempt, "approved")

    def test_cross_company_grant_reassignment_cannot_finalize(self):
        order, approval, attempt = self._approved_order()
        other_company = self.env["res.company"].create(
            {
                "name": "TXN03 Other Company",
                "pdp_tenant_id": self.env.company.pdp_tenant_id,
            }
        )
        other_delegator = self._user(
            "TXN03 Other Delegator",
            "txn03_other_delegator",
            self.env.ref("purchase.group_purchase_manager"),
        )
        other_delegator.write(
            {
                "company_id": other_company.id,
                "company_ids": [Command.set([other_company.id])],
            }
        )
        self.grant.write({"user_id": other_delegator.id})

        with self.assertRaisesRegex(
            UserError, "Delegation grant and purchase order belong to different companies"
        ):
            with self.env.cr.savepoint():
                self._public_confirm(order)

        self._assert_not_consumed(order, approval, attempt, "approved")
        self.assertEqual(self.grant.user_id.company_id, other_company)

    def test_final_protocol_variants_fail_closed(self):
        variants = (
            ("unknown schema", "cbi.intent_version", lambda value: "cbi.v99"),
            ("unknown intent proof version", "cbi.proof_version", lambda value: "v99"),
            ("unknown proof envelope", "delegation_proof", lambda value: "v99." + value.split(".", 1)[1]),
            ("unknown material field", "cbi.line_discount", lambda value: "1"),
            ("missing company", "cbi.company_id", lambda value: None),
            ("leading-zero ID", "cbi.company_id", lambda value: "0" + value),
            ("decimal minor units", "cbi.amount_minor", lambda value: value + ".0"),
            ("exponent minor units", "cbi.amount_minor", lambda value: value + "e0"),
            ("int64 overflow", "cbi.amount_minor", lambda value: str(1 << 63)),
            ("non-hex digest", "cbi.line_digest", lambda value: "g" + value[1:]),
            ("noncanonical timestamp", "cbi.record_write_version", lambda value: value[:-1] + "+00:00"),
            ("truncated proof signature", "delegation_proof", lambda value: value[:-1]),
        )
        for label, key, mutate in variants:
            with self.subTest(variant=label):
                order, approval, attempt = self._approved_order()
                binding = (approval.intent_hash, approval.one_time_id, approval.command_id)
                order_class = type(order)
                original = order_class._protected_delegated_request
                injected = []

                def malformed_request(record, intent, proof, issued_at, valid_until):
                    request = original(record, intent, proof, issued_at, valid_until)
                    context = request["context"]
                    previous = context.get(key)
                    value = mutate(previous)
                    self.assertNotEqual(value, previous)
                    if value is None:
                        del context[key]
                    else:
                        context[key] = value
                    injected.append(label)
                    return request

                with patch.object(order_class, "_protected_delegated_request", malformed_request):
                    with self.assertRaises(AccessError) as denied:
                        with self.env.cr.savepoint():
                            self._public_confirm(order)

                self.assertEqual(injected, [label])
                cause = denied.exception.__cause__
                self.assertIsInstance(cause, grpc.RpcError)
                self.assertEqual(cause.code(), grpc.StatusCode.PERMISSION_DENIED)
                self._assert_not_consumed(order, approval, attempt, "approved")
                self.assertEqual(
                    (approval.intent_hash, approval.one_time_id, approval.command_id), binding
                )

    def test_stored_canonical_encoding_variants_cannot_finalize(self):
        for label, mutate in (
            ("non-base64", lambda value: "%not-canonical%"),
            ("padded", lambda value: value + "="),
            ("truncated", lambda value: value[:-1]),
        ):
            with self.subTest(variant=label):
                order, approval, attempt = self._approved_order()
                binding = (approval.intent_hash, approval.one_time_id, approval.command_id)
                corrupted = mutate(approval.canonical_intent_b64)
                approval.write({"canonical_intent_b64": corrupted})
                approval.flush_recordset(["canonical_intent_b64"])
                with self.assertRaisesRegex(AccessError, "Stored approval intent has inconsistent binding"):
                    with self.env.cr.savepoint():
                        self._public_confirm(order)

                self._assert_not_consumed(order, approval, attempt, "approved")
                self.assertEqual(approval.canonical_intent_b64, corrupted)
                self.assertEqual(
                    (approval.intent_hash, approval.one_time_id, approval.command_id), binding
                )

    def test_v1_proof_cannot_downgrade_approved_final_route(self):
        order, approval, attempt = self._approved_order()
        grant_class = type(self.grant)
        original = grant_class.build_protected_intent

        def replace_v2_with_v1(grant, purchase_order, nonce):
            intent, _proof, fingerprint, issued_at, valid_until = original(
                grant, purchase_order, nonce
            )
            return (
                intent,
                "v1.testbed-2026." + "0" * 64,
                fingerprint,
                issued_at,
                valid_until,
            )

        with patch.object(grant_class, "build_protected_intent", replace_v2_with_v1):
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    self._public_confirm(order)

        self._assert_not_consumed(order, approval, attempt, "approved")

    def test_approval_capability_cannot_substitute_for_delegation_proof(self):
        order, approval, attempt = self._approved_order()
        grant_class = type(self.grant)
        original = grant_class.build_protected_intent

        def substitute_capability_signature(grant, purchase_order, nonce):
            intent, _proof, fingerprint, issued_at, valid_until = original(
                grant, purchase_order, nonce
            )
            return (
                intent,
                approval.capability_signature_b64,
                fingerprint,
                issued_at,
                valid_until,
            )

        with patch.object(
            grant_class, "build_protected_intent", substitute_capability_signature
        ):
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    self._public_confirm(order)

        self._assert_not_consumed(order, approval, attempt, "approved")

    def test_activity_changes_never_count_as_human_approval(self):
        for action in ("complete", "delete", "reassign"):
            with self.subTest(action=action):
                order, approval, attempt = self._approved_order(issue=False)
                activity = approval.activity_id
                if action == "complete":
                    activity.action_done()
                elif action == "delete":
                    activity.unlink()
                else:
                    activity.write({"user_id": self.creator.id})

                self.assertTrue(self._public_confirm(order))

                self._assert_not_consumed(order, approval, attempt, "pending")
                self.assertFalse(approval.one_time_id)
                self.assertFalse(approval.capability_signature_b64)

    def test_missing_approval_row_or_capability_fails_closed(self):
        for missing in ("approval row", "capability payload"):
            with self.subTest(missing=missing):
                order, approval, attempt = self._approved_order()
                if missing == "approval row":
                    approval.unlink()
                else:
                    approval.write({"capability_payload_b64": False})

                if missing == "approval row":
                    with self.assertRaises(AccessError):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                else:
                    self.assertTrue(self._public_confirm(order))
                    approval.invalidate_recordset()
                    self.assertEqual(approval.state, "invalidated")
                    self.assertEqual(
                        approval.terminal_reason, "capability_verification_failed"
                    )

                order.invalidate_recordset()
                attempt.invalidate_recordset()
                self.assertEqual(order.state, "to approve")
                self.assertEqual(attempt.state, "approval_required")
                self.assertNotEqual(attempt.result_state, "purchase")

    def test_partial_capability_fields_cannot_finalize(self):
        missing_fields = (
            "capability_payload_b64", "capability_signature_b64",
            "capability_version", "capability_purpose", "capability_algorithm",
            "capability_key_id", "one_time_id", "required_permission",
            "issued_at", "expires_at", "approver_user_id", "approver_subject",
        )
        for field in missing_fields:
            with self.subTest(missing=field):
                order, approval, attempt = self._approved_order()
                approval.write({field: False})
                approval.flush_recordset([field])
                approval.invalidate_recordset([field])
                self.assertFalse(approval[field])

                self.assertTrue(self._public_confirm(order))

                expected_state = "expired" if field == "expires_at" else "invalidated"
                self._assert_not_consumed(order, approval, attempt, expected_state)
                if field == "expires_at":
                    reason = "capability_expired"
                elif field in ("approver_user_id", "approver_subject"):
                    reason = "current_approver_authority_denied"
                else:
                    reason = "capability_verification_failed"
                self.assertEqual(approval.terminal_reason, reason)

    def test_issuance_revision_corruption_cannot_finalize(self):
        for corruption in ("null", "zero", "negative", "changed"):
            with self.subTest(corruption=corruption):
                order, approval, attempt = self._approved_order()
                original = approval.issuance_policy_revision
                self.assertGreater(original, 0, "Fixture requires a nonzero signed revision")
                value = {"null": None, "zero": 0, "negative": -1,
                         "changed": original + 1}[corruption]
                approval.flush_recordset()
                self.env.cr.execute(
                    "UPDATE pdp_approval_request SET issuance_policy_revision = %s "
                    "WHERE id = %s RETURNING issuance_policy_revision",
                    [value, approval.id],
                )
                self.assertEqual(self.env.cr.fetchone()[0], value)
                approval.invalidate_recordset(["issuance_policy_revision"], flush=False)

                self.assertTrue(self._public_confirm(order))

                self._assert_not_consumed(order, approval, attempt, "invalidated")
                self.assertEqual(approval.terminal_reason, "capability_verification_failed")

    def test_required_approval_bindings_cannot_be_null(self):
        order, approval, attempt = self._approved_order()
        required_fields = (
            "approval_id", "tenant_id", "company_id", "purchase_order_id",
            "authorization_attempt_id", "delegation_grant_id", "command_id",
            "intent_json", "canonical_intent_b64", "intent_hash", "state_witness", "state",
        )
        approval.flush_recordset()
        for field in required_fields:
            with self.subTest(missing=field):
                with self.assertRaises(errors.NotNullViolation) as rejected:
                    with self.env.cr.savepoint():
                        self.env.cr.execute(
                            sql.SQL("UPDATE pdp_approval_request SET {} = NULL WHERE id = %s")
                            .format(sql.Identifier(field)),
                            [approval.id],
                        )
                self.assertEqual(rejected.exception.diag.column_name, field)
                self._assert_not_consumed(order, approval, attempt, "approved")
                self.assertTrue(approval[field])

    def test_empty_approval_binding_text_cannot_finalize(self):
        for field in (
            "approval_id", "tenant_id", "command_id", "intent_json",
            "canonical_intent_b64", "intent_hash", "state_witness",
        ):
            with self.subTest(empty=field):
                order, approval, attempt = self._approved_order()
                approval.flush_recordset()
                self.env.cr.execute(
                    sql.SQL("UPDATE pdp_approval_request SET {} = %s WHERE id = %s")
                    .format(sql.Identifier(field)),
                    ["", approval.id],
                )
                approval.invalidate_recordset([field], flush=False)
                self.assertFalse(approval[field])

                if field == "approval_id":
                    self.assertTrue(self._public_confirm(order))
                    self._assert_not_consumed(order, approval, attempt, "invalidated")
                    self.assertEqual(approval.terminal_reason, "capability_verification_failed")
                else:
                    with self.assertRaises(AccessError):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                    self._assert_not_consumed(order, approval, attempt, "approved")
                self.assertFalse(approval[field])

    def test_duplicate_approval_for_one_attempt_is_rejected(self):
        order, approval, attempt = self._approved_order()
        duplicate = {
            "approval_id": approval.approval_id + "-duplicate",
            "tenant_id": approval.tenant_id,
            "company_id": approval.company_id.id,
            "purchase_order_id": order.id,
            "authorization_attempt_id": attempt.id,
            "delegation_grant_id": approval.delegation_grant_id.id,
            "command_id": approval.command_id,
            "intent_json": approval.intent_json,
            "canonical_intent_b64": approval.canonical_intent_b64,
            "intent_hash": approval.intent_hash,
            "state_witness": approval.state_witness,
        }

        with self.assertRaises(IntegrityError):
            with self.env.cr.savepoint():
                self.env["pdp.approval.request"].sudo().create(duplicate)

        self.assertEqual(
            self.env["pdp.approval.request"].sudo().search_count(
                [("authorization_attempt_id", "=", attempt.id)]
            ),
            1,
        )
        self._assert_not_consumed(order, approval, attempt, "approved")

    def test_unknown_approval_state_cannot_finalize(self):
        order, approval, attempt = self._approved_order()
        self.env.flush_all()
        self.env.cr.execute(
            "UPDATE pdp_approval_request SET state = %s WHERE id = %s",
            ["unexpected", approval.id],
        )
        approval.invalidate_recordset(["state"], flush=False)
        self.assertEqual(approval.state, "unexpected")

        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                self._public_confirm(order)

        self._assert_not_consumed(order, approval, attempt, "unexpected")
        self.assertFalse(approval.terminal_at)

    def test_revoked_grant_invalidates_without_consumption(self):
        order, approval, attempt = self._approved_order()
        self.grant.action_revoke()

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "delegation_revoked")

    def test_erp_tombstone_blocks_even_before_pdp_publication(self):
        order, approval, attempt = self._approved_order()
        self.env.cr.execute(
            "INSERT INTO pdp_delegation_fence_v1 VALUES (%s, %s, true)",
            [approval.tenant_id, str(self.grant.id)],
        )
        # PDP and local grant still permit the action: the durable ERP fence is
        # authoritative even if the second (PDP publication) write failed.
        self.assertEqual(self.grant.state, "active")
        self.assertTrue(self._public_confirm(order))
        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "delegation_revoked")

    def test_final_requires_pdp_fence_acknowledgement(self):
        for scope in (None, "odoo-revocation.v1:wrong-database"):
            with self.subTest(scope=scope):
                order, approval, attempt = self._approved_order()
                stub = pdp_client.get_pdp_client()._get_stub()
                original = stub.CheckAccess

                def corrupt_ack(*args, **kwargs):
                    response = original(*args, **kwargs)
                    if scope is None:
                        response.advice.pop("erp.revocation_fence", None)
                    else:
                        response.advice["erp.revocation_fence"] = scope
                    return response

                with patch.object(stub, "CheckAccess", corrupt_ack):
                    with self.assertRaisesRegex(AccessError, "required ERP revocation fence"):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                self._assert_not_consumed(order, approval, attempt, "approved")

    def test_current_approver_policy_denial_prevents_consumption(self):
        order, approval, attempt = self._approved_order()
        self.approver.write({"pdp_department": "Finance"})

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(
            approval.terminal_reason, "current_approver_authority_denied"
        )

    def test_expired_capability_cannot_reach_final_state(self):
        order, approval, attempt = self._approved_order()
        approval.write(
            {"expires_at": fields.Datetime.now() - timedelta(seconds=1)}
        )

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "expired")
        self.assertEqual(approval.terminal_reason, "capability_expired")

    def test_tampered_capability_cannot_reach_final_state(self):
        order, approval, attempt = self._approved_order()
        approval.write({"capability_signature_b64": "AAAA"})

        self.assertTrue(self._public_confirm(order))

        self._assert_not_consumed(order, approval, attempt, "invalidated")
        self.assertEqual(approval.terminal_reason, "capability_verification_failed")

    def test_current_agent_policy_denial_prevents_consumption(self):
        order, approval, attempt = self._approved_order()
        original = pdp_client._client

        class DenyAgentConfirmation:
            def __init__(self, denial_obligations):
                self.denial_obligations = denial_obligations

            def check_access(self, tenant, subject, action, resource, context, token):
                if action == "action:CONFIRM_PURCHASE_ORDER":
                    return "DENY", self.denial_obligations, {}
                return original.check_access(
                    tenant, subject, action, resource, context, token
                )

            def verify_approval_capability(self, tenant, capability, token):
                return original.verify_approval_capability(tenant, capability, token)

        try:
            for obligations in (
                [],
                [{"type": "REQUIRE_HUMAN_APPROVAL", "message": "review", "payload": {}}],
            ):
                with self.subTest(obligations=obligations):
                    pdp_client._client = DenyAgentConfirmation(obligations)
                    with self.assertRaises(AccessError):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                    self._assert_not_consumed(order, approval, attempt, "approved")
        finally:
            pdp_client._client = original

    def test_business_failure_after_approval_consumption_rolls_back(self):
        order, approval, attempt = self._approved_order()
        order_class = type(order)
        original_approve = order_class.button_approve

        def fail_after_business_transition(record):
            result = original_approve(record)
            self.assertEqual(record.state, "purchase")
            raise RuntimeError("injected failure after PO transition")

        with patch.object(order_class, "button_approve", fail_after_business_transition):
            with self.assertRaisesRegex(RuntimeError, "injected failure"):
                with self.env.cr.savepoint():
                    self._public_confirm(order)

        self._assert_not_consumed(order, approval, attempt, "approved")
        self.assertTrue(self._public_confirm(order))
        self.assertEqual(order.state, "purchase")
        self.assertEqual(approval.state, "consumed")
        self.assertEqual(attempt.state, "executed")

    def test_final_agent_pdp_outage_preserves_retryable_approval(self):
        order, approval, attempt = self._approved_order()
        original = pdp_client._client

        class LateAgentOutage:
            def check_access(self, tenant, subject, action, resource, context, token):
                if action == "action:CONFIRM_PURCHASE_ORDER":
                    raise pdp_client.PDPUnavailableError("injected final PDP outage")
                return original.check_access(
                    tenant, subject, action, resource, context, token
                )

            def verify_approval_capability(self, tenant, capability, token):
                return original.verify_approval_capability(tenant, capability, token)

        try:
            pdp_client._client = LateAgentOutage()
            with self.assertRaises(pdp_client.PDPUnavailableError):
                with self.env.cr.savepoint():
                    self._public_confirm(order)
        finally:
            pdp_client._client = original

        self._assert_not_consumed(order, approval, attempt, "approved")
        self.assertTrue(self._public_confirm(order))
        self.assertEqual(order.state, "purchase")

    def test_final_capability_verifier_transport_outage_preserves_approval(self):
        order, approval, attempt = self._approved_order()
        original = pdp_client._client
        unavailable = pdp_client.SafePDPClient(target="127.0.0.1:1", timeout=0.1)
        try:
            pdp_client._client = unavailable
            with self.assertRaises(pdp_client.PDPUnavailableError):
                with self.env.cr.savepoint():
                    self._public_confirm(order)
        finally:
            pdp_client._client = original
            unavailable._channel.close()

        self._assert_not_consumed(order, approval, attempt, "approved")
        self.assertTrue(self._public_confirm(order))
        self.assertEqual(order.state, "purchase")

    def test_final_policy_revision_fence_faults_preserve_approval(self):
        order, approval, attempt = self._approved_order()
        original = type(order)._lock_current_authority
        for fault in ("missing", "malformed", "mixed", "stale", "pending"):
            with self.subTest(fault=fault):
                def inject(record, revisions):
                    values = list(revisions)
                    if fault == "missing":
                        values[-1] = None
                    elif fault == "malformed":
                        values[-1] = "1.0"
                    elif fault == "mixed":
                        values[0] = str(int(values[-1]) + 1)
                    elif fault == "stale":
                        record.env.cr.execute("UPDATE pdp_policy_fence_v1 SET revision=revision+1 WHERE tenant_id=%s", [record._tenant_id()])
                    else:
                        record.env.cr.execute("UPDATE pdp_policy_fence_v1 SET ready=false WHERE tenant_id=%s", [record._tenant_id()])
                    return original(record, values)
                with patch.object(type(order), "_lock_current_authority", inject):
                    with self.assertRaises(AccessError):
                        with self.env.cr.savepoint():
                            self._public_confirm(order)
                self._assert_not_consumed(order, approval, attempt, "approved")
