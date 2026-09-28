"""Public Odoo entry-point tests for delegated PO final transitions."""

from datetime import timedelta

from odoo import Command, fields
from odoo.api import call_kw
from odoo.exceptions import AccessError
from odoo.tests import TransactionCase, tagged


@tagged("post_install", "-at_install")
class TestTransitionEntrypoints(TransactionCase):
    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        manager = cls.env.ref("purchase.group_purchase_manager")
        cls.env.company.write(
            {
                "pdp_tenant_id": cls._tenant_id(),
                "po_double_validation": "one_step",
            }
        )
        cls.env.user.write({"pdp_department": "Procurement"})
        cls.creator = cls._create_user("TXN01 Creator", "txn01_creator", manager)
        cls.approver = cls._create_user(
            "TXN01 Approver", "pdp_e2e_approver", manager
        )
        cls.vendor = cls.env["res.partner"].create(
            {"name": "TXN01 Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "TXN01 Product", "purchase_ok": True}
        )
        cls.env.cr.execute(
            """
            SELECT setval(
                pg_get_serial_sequence('pdp_delegation_grant', 'id'),
                1700000000 +
                    (EXTRACT(EPOCH FROM clock_timestamp())::bigint % 50000000),
                true
            )
            """
        )

    @classmethod
    def _tenant_id(cls):
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
                "max_amount": 2000,
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
            }
        )
        self.grant.action_activate()

    def _order(self, amount):
        return self.env["purchase.order"].with_user(self.creator).sudo().create(
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

    def _public_call(self, method, order, *args):
        model = self.env["purchase.order"].with_user(self.creator)
        return call_kw(model, method, [[order.id], *args], {})

    def _assert_unchanged_draft(self, order):
        order.invalidate_recordset()
        self.assertEqual(order.state, "draft")
        self.assertEqual(order.pdp_status, "pending")
        self.assertFalse(order.pdp_delegation_nonce)
        self.assertFalse(
            self.env["pdp.authorization.attempt"].sudo().search(
                [("business_model", "=", order._name), ("business_res_id", "=", order.id)]
            )
        )

    def test_public_final_methods_cannot_bypass_confirmation(self):
        for method in ("button_approve", "button_done", "button_unlock"):
            with self.subTest(method=method):
                order = self._order(1000)
                with self.assertRaises(AccessError):
                    with self.env.cr.savepoint():
                        self._public_call(method, order)
                self._assert_unchanged_draft(order)

    def test_public_write_cannot_forge_final_state(self):
        for target in ("purchase", "done"):
            with self.subTest(target=target):
                order = self._order(1000)
                with self.assertRaises(AccessError):
                    with self.env.cr.savepoint():
                        self._public_call("write", order, {"state": target})
                self._assert_unchanged_draft(order)

    def test_serializable_context_value_cannot_forge_internal_guard(self):
        order = self._order(1000)
        model = self.env["purchase.order"].with_user(self.creator).with_context(
            _pdp_authorized_final_transition=True
        )

        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                call_kw(model, "button_approve", [[order.id]], {})

        self._assert_unchanged_draft(order)

    def test_delegated_scope_stays_sticky_after_markers_are_cleared(self):
        order = self._order(1000)
        self.assertTrue(order.pdp_delegated_scope)

        self._public_call(
            "write",
            order,
            {
                "delegation_grant_id": False,
                "ai_agent_id": False,
                "delegated_by_id": False,
                "pdp_delegated_scope": False,
            },
        )
        order.invalidate_recordset()
        self.assertTrue(order.pdp_delegated_scope)
        self.assertFalse(order.delegation_grant_id)

        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                self._public_call("button_approve", order)

        self._assert_unchanged_draft(order)

    def test_supported_button_confirm_route_still_executes_allow_once(self):
        order = self._order(1000)

        self.assertTrue(self._public_call("button_confirm", order))

        order.invalidate_recordset()
        attempt = self.env["pdp.authorization.attempt"].sudo().search(
            [("business_model", "=", order._name), ("business_res_id", "=", order.id)]
        ).ensure_one()
        self.assertEqual(order.state, "purchase")
        self.assertEqual(attempt.state, "executed")

        self._public_call("button_done", order)
        order.invalidate_recordset()
        self.assertEqual(order.state, "done")
        self._public_call("button_unlock", order)
        order.invalidate_recordset()
        self.assertEqual(order.state, "purchase")

    def test_issued_capability_does_not_enable_direct_button_approve(self):
        order = self._order(2500)
        self.assertTrue(self._public_call("button_confirm", order))
        approval = self.env["pdp.approval.request"].sudo().search(
            [("purchase_order_id", "=", order.id)]
        ).ensure_one()
        self.assertTrue(
            approval.with_user(self.approver).action_issue_capability()
        )

        with self.assertRaises(AccessError):
            with self.env.cr.savepoint():
                self._public_call("button_approve", order)

        order.invalidate_recordset()
        approval.invalidate_recordset()
        self.assertEqual(order.state, "to approve")
        self.assertEqual(approval.state, "approved")
        self.assertFalse(approval.terminal_reason)
        self.assertFalse(approval.terminal_at)
