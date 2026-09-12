import json
import os
from datetime import timedelta

from odoo import Command, fields
from odoo.exceptions import AccessError
from odoo.tests import TransactionCase, tagged

from ..models import pdp_client
from ..pdp_protocol import issue_jwt_from_environment


@tagged("post_install", "-at_install")
class TestOdooPDPRealBoundary(TransactionCase):
    """Real Odoo ORM -> generated gRPC client -> live Go PDP cases."""

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        tenant_id = os.environ["PDP_TENANT_ID"]
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
            }
        )
        cls.vendor = cls.env["res.partner"].create(
            {"name": "PDP E2E Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "PDP E2E Product", "purchase_ok": True}
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

    def _assert_rolled_back_deny(self, order):
        order.invalidate_recordset()
        self.assertEqual(order.state, "draft")
        self.assertFalse(order.pdp_delegation_nonce)
        self.assertFalse(self._attempts(order))

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

        self.assertTrue(order.button_confirm())
        self.assertEqual(len(order.activity_ids), 1)
        self.assertEqual(len(self._attempts(order)), 1)

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
