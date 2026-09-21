import base64
import json
import os
from datetime import timedelta

from odoo import Command, fields
from odoo.exceptions import AccessError
from odoo.tests import TransactionCase, tagged

from ..cbi_protocol import (
    amount_to_minor_units,
    canonical_business_intent_bytes,
    canonical_business_intent_hash,
    minor_units_to_decimal,
)
from ..models import pdp_client
from ..pdp_protocol import issue_jwt_from_environment


@tagged("post_install", "-at_install")
class TestOdooPDPRealBoundary(TransactionCase):
    """Real Odoo ORM -> generated gRPC client -> live Go PDP cases."""

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        tenant_id = os.environ["PDP_TENANT_ID"]
        purchase_manager = cls.env.ref("purchase.group_purchase_manager")
        cls.env.company.write(
            {
                "pdp_tenant_id": tenant_id,
                "po_double_validation": "one_step",
            }
        )
        cls.env.user.write({"pdp_department": "Procurement"})
        cls.creator = cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "PDP E2E Creator",
                "login": "pdp_e2e_creator",
                "email": "pdp-e2e-creator@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "pdp_department": "Procurement",
                "groups_id": [Command.link(purchase_manager.id)],
            }
        )
        cls.approver = cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "PDP E2E Approver",
                "login": "pdp_e2e_approver",
                "email": "pdp-e2e-approver@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "groups_id": [Command.link(purchase_manager.id)],
                "pdp_department": "Procurement",
            }
        )
        cls.unlisted_manager = cls.env["res.users"].with_context(
            no_reset_password=True
        ).create(
            {
                "name": "PDP E2E Unlisted Manager",
                "login": "pdp_e2e_unlisted_manager",
                "email": "pdp-e2e-unlisted-manager@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "groups_id": [Command.link(purchase_manager.id)],
                "pdp_department": "Procurement",
            }
        )
        cls.wrong_role = cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "PDP E2E Wrong Role",
                "login": "pdp_e2e_wrong_role",
                "email": "pdp-e2e-wrong-role@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "pdp_department": "Procurement",
            }
        )
        cls.other_company = cls.env["res.company"].create(
            {"name": "PDP E2E Other Company", "pdp_tenant_id": "tenant-other"}
        )
        cls.cross_tenant_manager = cls.env["res.users"].with_context(
            no_reset_password=True
        ).create(
            {
                "name": "PDP E2E Cross Tenant Manager",
                "login": "pdp_e2e_cross_tenant_manager",
                "email": "pdp-e2e-cross-tenant@example.test",
                "company_id": cls.other_company.id,
                "company_ids": [Command.set([cls.other_company.id])],
                "groups_id": [Command.link(purchase_manager.id)],
                "pdp_department": "Procurement",
            }
        )
        cls.vendor = cls.env["res.partner"].create(
            {"name": "PDP E2E Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "PDP E2E Product", "purchase_ok": True}
        )
        cls.env.cr.execute(
            """
            SELECT setval(
                pg_get_serial_sequence('pdp_delegation_grant', 'id'),
                EXTRACT(EPOCH FROM clock_timestamp())::bigint,
                true
            )
            """
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

    def _order(self, amount, creator=None):
        creator = creator or self.creator
        model = self.env["purchase.order"].with_user(creator).sudo()
        return model.create(
            {
                "partner_id": self.vendor.id,
                "company_id": self.env.company.id,
                "pdp_department": "Procurement",
                "ai_agent_id": self.grant.agent_id,
                "delegated_by_id": self.grant.user_id.id,
                "delegation_grant_id": self.grant.id,
                "order_line": [
                    Command.create(
                        {
                            "name": self.product.display_name,
                            "product_id": self.product.id,
                            "product_qty": 1,
                            "product_uom": self.product.uom_po_id.id,
                            "price_unit": amount,
                            "date_planned": fields.Datetime.now(),
                        }
                    )
                ],
            }
        )

    def _attempts(self, order):
        return self.env["pdp.authorization.attempt"].sudo().search(
            [("business_model", "=", order._name), ("business_res_id", "=", order.id)]
        )

    def _approvals(self, order):
        return self.env["pdp.approval.request"].sudo().search(
            [("purchase_order_id", "=", order.id)]
        )

    def _assert_rolled_back_deny(self, order):
        order.invalidate_recordset()
        self.assertEqual(order.state, "draft")
        self.assertFalse(order.pdp_delegation_nonce)
        self.assertFalse(self._attempts(order))

    def test_v2_request_is_reconstructed_from_persisted_order(self):
        order = self._order(1000.25)
        request, attempt, created = order._delegated_request()
        self.env.cr.execute(
            """
            SELECT po.amount_total::text, currency.decimal_places
              FROM purchase_order AS po
              JOIN res_currency AS currency ON currency.id = po.currency_id
             WHERE po.id = %s
            """,
            [order.id],
        )
        amount_total, currency_scale = self.env.cr.fetchone()
        expected_minor = amount_to_minor_units(amount_total, currency_scale)
        self.assertTrue(created)
        self.assertEqual(attempt.state, "pending")
        self.assertEqual(request["action"], "action:CONFIRM_PURCHASE_ORDER")
        self.assertEqual(request["context"]["cbi.amount_minor"], str(expected_minor))
        self.assertEqual(
            request["context"]["amount"],
            minor_units_to_decimal(expected_minor, currency_scale),
        )
        self.assertEqual(request["context"]["cbi.resource_id"], str(order.id))
        self.assertTrue(request["context"]["delegation_proof"].startswith("v2."))

    def test_allow_is_executed_once(self):
        order = self._order(1000)
        self.assertTrue(order.button_confirm())
        self.assertEqual(order.state, "purchase")
        attempts = self._attempts(order)
        self.assertEqual(len(attempts), 1)
        self.assertEqual(attempts.state, "executed")

        self.assertTrue(order.button_confirm())
        self.assertEqual(len(self._attempts(order)), 1)

    def test_approval_obligation_commits_once_without_rollback(self):
        order = self._order(2500)
        self.assertTrue(order.button_confirm())
        self.assertEqual(order.state, "to approve")
        self.assertEqual(self._attempts(order).state, "approval_required")
        activities = order.activity_ids
        self.assertEqual(len(activities), 1)
        approvals = self._approvals(order)
        self.assertEqual(len(approvals), 1)
        pending = approvals.ensure_one()
        intent = json.loads(pending.intent_json)
        padding = "=" * (-len(pending.canonical_intent_b64) % 4)
        self.assertEqual(pending.state, "pending")
        self.assertEqual(pending.authorization_attempt_id, self._attempts(order))
        self.assertEqual(pending.activity_id, activities)
        self.assertEqual(pending.requested_approver_user_id, self.approver)
        self.assertEqual(activities.user_id, self.approver)
        self.assertEqual(pending.tenant_id, order._tenant_id())
        self.assertEqual(pending.company_id, order.company_id)
        self.assertEqual(pending.delegation_grant_id, self.grant)
        self.assertEqual(pending.command_id, order.pdp_delegation_nonce)
        self.assertEqual(intent["record_state"], "to approve")
        self.assertEqual(intent["command_id"], order.pdp_delegation_nonce)
        self.assertEqual(pending.state_witness, intent["state_witness"])
        self.assertEqual(pending.intent_hash, canonical_business_intent_hash(intent))
        self.assertEqual(
            base64.urlsafe_b64decode(pending.canonical_intent_b64 + padding),
            canonical_business_intent_bytes(intent),
        )
        self.assertNotIn(order.state, ("purchase", "done"))

        self.assertTrue(order.button_confirm())
        self.assertEqual(len(order.activity_ids), 1)
        self.assertEqual(len(self._attempts(order)), 1)
        self.assertEqual(len(self._approvals(order)), 1)

    def test_approver_authority_and_sod_fail_closed(self):
        order = self._order(2500)
        self.assertTrue(order.button_confirm())
        pending = self._approvals(order).ensure_one()

        authority = pending.with_user(self.approver).check_current_approver()
        self.assertEqual(authority["approver_user_id"], self.approver.id)
        self.assertEqual(authority["approver_subject"], "user:pdp_e2e_approver")

        for user in (
            self.env.user,
            self.creator,
            self.wrong_role,
            self.unlisted_manager,
            self.cross_tenant_manager,
        ):
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    pending.with_user(user).check_current_approver()

        tenant_id = order.company_id.pdp_tenant_id
        agent_subject = self.grant.agent_id
        agent_token = issue_jwt_from_environment(agent_subject, tenant_id)
        decision, _obligations, _advice = pdp_client.get_pdp_client().check_access(
            tenant_id,
            agent_subject,
            "action:APPROVE_PURCHASE_ORDER",
            "purchase_order:%s" % order.id,
            {
                "approval_state": "pending",
                "resource.department": order.pdp_department,
            },
            agent_token,
        )
        self.assertEqual(decision, "DENY")

        original = pdp_client._client
        unavailable = pdp_client.SafePDPClient("127.0.0.1:1", timeout=0.05)
        pdp_client._client = unavailable
        try:
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    pending.with_user(self.approver).check_current_approver()
        finally:
            unavailable._channel.close()
            pdp_client._client = original

        order.invalidate_recordset()
        pending.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(pending.state, "pending")
        self.assertEqual(len(self._approvals(order)), 1)
        self.assertEqual(len(order.activity_ids), 1)

    def test_approval_capability_is_issued_verified_and_idempotent(self):
        order = self._order(2500)
        self.assertTrue(order.button_confirm())
        pending = self._approvals(order).ensure_one()

        self.assertTrue(pending.with_user(self.approver).action_issue_capability())
        pending.invalidate_recordset()
        order.invalidate_recordset()
        self.assertEqual(pending.state, "approved")
        self.assertEqual(order.state, "to approve")
        self.assertEqual(pending.approver_user_id, self.approver)
        self.assertEqual(pending.approver_subject, "user:pdp_e2e_approver")
        self.assertEqual(pending.capability_version, "ac.v1")
        self.assertEqual(pending.capability_purpose, "odoo.purchase_order.confirm")
        self.assertEqual(pending.capability_algorithm, "HS256")
        self.assertEqual(pending.capability_key_id, "approval-testbed-2026")
        self.assertEqual(
            pending.required_permission, "approval:purchase_order.confirm"
        )
        self.assertGreaterEqual(pending.issuance_policy_revision, 1)
        self.assertTrue(pending.one_time_id)
        self.assertTrue(pending.capability_payload_b64)
        self.assertTrue(pending.capability_signature_b64)
        self.assertEqual(len(pending.capability_fingerprint), 64)
        self.assertNotIn(order.state, ("purchase", "done"))

        original = (
            pending.one_time_id,
            pending.capability_payload_b64,
            pending.capability_signature_b64,
            pending.capability_fingerprint,
        )
        self.assertTrue(pending.with_user(self.approver).action_issue_capability())
        pending.invalidate_recordset()
        self.assertEqual(
            original,
            (
                pending.one_time_id,
                pending.capability_payload_b64,
                pending.capability_signature_b64,
                pending.capability_fingerprint,
            ),
        )

        token = pending._approver_token(
            self.approver, "user:pdp_e2e_approver"
        )
        tampered = pending._stored_capability()
        tampered["intent_hash"] = "f" * 64
        with self.assertRaises(AccessError):
            pdp_client.get_pdp_client().verify_approval_capability(
                pending.tenant_id, tampered, token
            )
        confused = pending._stored_capability()
        confused["key_id"] = os.environ["PDP_DELEGATION_ACTIVE_KID"]
        with self.assertRaises(AccessError):
            pdp_client.get_pdp_client().verify_approval_capability(
                pending.tenant_id, confused, token
            )

        pending.invalidate_recordset()
        order.invalidate_recordset()
        self.assertEqual(pending.state, "approved")
        self.assertEqual(order.state, "to approve")
        self.assertEqual(len(self._approvals(order)), 1)

    def test_sod_hard_deny_rolls_back_attempt_and_nonce(self):
        order = self._order(1000, self.env.user)
        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                order.button_confirm()
        self._assert_rolled_back_deny(order)

    def test_tampered_signing_secret_fails_closed(self):
        order = self._order(1000)
        original = os.environ["PDP_DELEGATION_KEYS_JSON"]
        key_id = os.environ["PDP_DELEGATION_ACTIVE_KID"]
        os.environ["PDP_DELEGATION_KEYS_JSON"] = json.dumps(
            {key_id: "wrong-test-secret-that-is-still-at-least-32-characters"}
        )
        try:
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    order.button_confirm()
        finally:
            os.environ["PDP_DELEGATION_KEYS_JSON"] = original
        self._assert_rolled_back_deny(order)

    def test_pdp_outage_fails_closed_without_consuming_nonce(self):
        order = self._order(1000)
        original = pdp_client._client
        unavailable = pdp_client.SafePDPClient("127.0.0.1:1", timeout=0.05)
        pdp_client._client = unavailable
        try:
            with self.assertRaises(AccessError):
                with self.env.cr.savepoint():
                    order.button_confirm()
        finally:
            unavailable._channel.close()
            pdp_client._client = original
        self._assert_rolled_back_deny(order)

    def test_nonce_replay_for_different_order_fails_closed(self):
        first = self._order(1000)
        self.assertTrue(first.button_confirm())
        self.assertEqual(first.state, "purchase")
        self.assertEqual(len(self._attempts(first)), 1)

        replay = self._order(1000)
        replay.write({"pdp_delegation_nonce": first.pdp_delegation_nonce})
        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                replay.button_confirm()
        replay.invalidate_recordset()
        self.assertEqual(replay.state, "draft")
        self.assertFalse(self._attempts(replay))
        self.assertEqual(len(self._attempts(first)), 1)

    def test_revoked_grant_is_denied_by_live_pdp(self):
        order = self._order(1000)
        tenant_id = self.env.company.pdp_tenant_id
        revoked_by = "user:%s" % self.env.user.login
        token = issue_jwt_from_environment(
            revoked_by, tenant_id, permissions=("delegation:revoke",)
        )
        pdp_client.get_pdp_client().revoke_delegation(
            tenant_id, str(self.grant.id), revoked_by, token, "Odoo E2E revoke"
        )
        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                order.button_confirm()
        self._assert_rolled_back_deny(order)
