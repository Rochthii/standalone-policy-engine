"""E2E invalidation cases for issued ApprovalCapability v1 records."""

from datetime import timedelta

from odoo import Command, fields
from odoo.exceptions import AccessError
from odoo.tests import TransactionCase, tagged

from ..models import pdp_client


@tagged("post_install", "-at_install")
class TestApprovalInvalidation(TransactionCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        manager = cls.env.ref("purchase.group_purchase_manager")
        cls.manager = manager
        cls.env.company.write(
            {
                "pdp_tenant_id": cls._pdp_tenant_id(),
                "po_double_validation": "one_step",
            }
        )
        cls.env.user.write({"pdp_department": "Procurement"})
        cls.creator = cls._create_user(
            "APP04 Creator", "app04_creator", manager
        )
        cls.approver = cls._create_user(
            "APP04 Approver", "pdp_e2e_approver", manager
        )
        cls.vendor = cls.env["res.partner"].create(
            {"name": "APP04 Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "APP04 Product", "purchase_ok": True}
        )
        cls.env.cr.execute(
            """
            SELECT setval(
                pg_get_serial_sequence('pdp_delegation_grant', 'id'),
                1900000000 +
                    (EXTRACT(EPOCH FROM clock_timestamp())::bigint % 100000000),
                true
            )
            """
        )

    @classmethod
    def _pdp_tenant_id(cls):
        import os

        return os.environ["PDP_TENANT_ID"]

    @classmethod
    def _create_user(cls, name, login, manager):
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
                "currency_id": self.env.company.currency_id.id,
                "max_amount": 5000,
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
            }
        )
        self.grant.action_activate()

    def _approved_order(self):
        order = self.env["purchase.order"].with_user(self.creator).sudo().create(
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
                            "price_unit": 2500,
                            "date_planned": fields.Datetime.now(),
                        }
                    )
                ],
            }
        )
        self.assertTrue(order.button_confirm())
        approval = self.env["pdp.approval.request"].sudo().search(
            [("purchase_order_id", "=", order.id)]
        ).ensure_one()
        self.assertEqual(approval.requested_approver_user_id, self.approver)
        self.assertTrue(
            approval.with_user(self.approver).action_issue_capability()
        )
        approval.invalidate_recordset()
        self.assertEqual(approval.state, "approved")
        return order, approval

    def _assert_non_final(self, order, approval, expected_state, reason):
        order.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertNotIn(order.state, ("purchase", "done"))
        self.assertEqual(approval.state, expected_state)
        self.assertEqual(approval.terminal_reason, reason)
        self.assertTrue(approval.terminal_at)

    def test_changed_intent_invalidates_without_business_effect(self):
        order, approval = self._approved_order()
        order.order_line.write({"price_unit": 2600})

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(order, approval, "invalidated", "intent_changed")

    def test_unchanged_approval_remains_usable_and_non_final(self):
        order, approval = self._approved_order()

        self.assertTrue(approval._revalidate_approved_capability())

        order.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(approval.state, "approved")
        self.assertFalse(approval.terminal_reason)
        self.assertFalse(approval.terminal_at)

    def test_expired_capability_becomes_terminal_without_business_effect(self):
        order, approval = self._approved_order()
        approval.sudo().write(
            {"expires_at": fields.Datetime.now() - timedelta(seconds=1)}
        )

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(order, approval, "expired", "capability_expired")

    def test_revoked_delegation_invalidates_without_business_effect(self):
        order, approval = self._approved_order()
        self.grant.action_revoke()

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(order, approval, "invalidated", "delegation_revoked")

    def test_expired_delegation_invalidates_without_business_effect(self):
        order, approval = self._approved_order()
        self.grant.sudo().write(
            {"valid_until": fields.Datetime.now() - timedelta(seconds=1)}
        )

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(order, approval, "invalidated", "delegation_expired")

    def test_terminal_states_cannot_be_reopened(self):
        for state in ("rejected", "invalidated", "expired", "consumed"):
            with self.subTest(state=state):
                order, approval = self._approved_order()
                terminal_at = fields.Datetime.now()
                reason = "test_reserved_%s_state" % state
                approval.sudo().write(
                    {
                        "state": state,
                        "terminal_reason": reason,
                        "terminal_at": terminal_at,
                    }
                )

                with self.assertRaises(AccessError):
                    approval._revalidate_approved_capability()

                self._assert_non_final(order, approval, state, reason)
                self.assertEqual(approval.terminal_at, terminal_at)

    def test_lost_approver_role_invalidates_without_business_effect(self):
        order, approval = self._approved_order()
        self.approver.write({"groups_id": [Command.unlink(self.manager.id)]})

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(
            order,
            approval,
            "invalidated",
            "current_approver_authority_denied",
        )

    def test_current_pdp_policy_denial_invalidates_without_business_effect(self):
        order, approval = self._approved_order()
        self.approver.write({"pdp_department": "Finance"})

        self.assertFalse(approval._revalidate_approved_capability())

        self._assert_non_final(
            order,
            approval,
            "invalidated",
            "current_approver_authority_denied",
        )

    def test_pdp_outage_fails_closed_without_permanent_invalidation(self):
        order, approval = self._approved_order()
        original = pdp_client._client
        unavailable = pdp_client.SafePDPClient("127.0.0.1:1", timeout=0.05)
        pdp_client._client = unavailable
        try:
            with self.assertRaises(pdp_client.PDPUnavailableError):
                approval._revalidate_approved_capability()
        finally:
            unavailable._channel.close()
            pdp_client._client = original

        order.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(approval.state, "approved")
        self.assertFalse(approval.terminal_reason)
        self.assertFalse(approval.terminal_at)

    def test_final_preparation_returns_locked_unchanged_intent_only(self):
        order, approval = self._approved_order()

        current = approval._prepare_final_intent()

        self.assertEqual(current, approval._stored_intent())
        order.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(approval.state, "approved")

    def test_final_preparation_invalidates_changed_line_without_final_effect(self):
        order, approval = self._approved_order()
        order.order_line.write({"price_unit": 2600})

        self.assertFalse(approval._prepare_final_intent())

        self._assert_non_final(order, approval, "invalidated", "intent_changed")

    def test_final_preparation_invalidates_changed_vendor_without_final_effect(self):
        order, approval = self._approved_order()
        other_vendor = self.env["res.partner"].create(
            {"name": "TXN02 Replacement Vendor", "supplier_rank": 1}
        )
        order.write({"partner_id": other_vendor.id})

        self.assertFalse(approval._prepare_final_intent())

        self._assert_non_final(order, approval, "invalidated", "intent_changed")

    def test_final_preparation_rejects_mismatched_attempt_without_final_effect(self):
        order, approval = self._approved_order()
        attempt = approval.authorization_attempt_id
        attempt.write({"state": "executed"})

        with self.assertRaises(AccessError):
            approval._prepare_final_intent()

        order.invalidate_recordset()
        approval.invalidate_recordset()
        attempt.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(approval.state, "approved")
        self.assertEqual(attempt.state, "executed")
