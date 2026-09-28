"""Privileged ABC experiment adapters; not an addon, RPC endpoint or runtime flag."""
from contextlib import nullcontext
from unittest.mock import patch

from odoo.exceptions import AccessError
from odoo.addons.pdp_authorizer.models import purchase_order
from odoo.addons.pdp_authorizer.models.pdp_client import SafePDPClient, get_pdp_client
from odoo.addons.pdp_authorizer.pdp_protocol import issue_jwt_from_environment
from odoo.addons.pdp_authorizer.cbi_protocol import amount_to_minor_units, minor_units_to_decimal


def native_confirm(order):
    # Deliberate ablation inside this offline harness ONLY: bypass the delegated
    # PEP, retaining Odoo ACLs/record rules/workflow. C never calls this helper.
    authorized = order.with_context(**{
        purchase_order._FINAL_TRANSITION_CONTEXT_KEY: purchase_order._FINAL_TRANSITION_SENTINEL})
    if order.state == "to approve":
        return authorized.button_approve()
    return super(purchase_order.PurchaseOrder, authorized).button_confirm()


def policy_only(order, client):
    order.env.cr.execute("SELECT amount_total::text FROM purchase_order WHERE id=%s", [order.id])
    amount = order.env.cr.fetchone()[0]
    scale = order.currency_id.decimal_places
    amount = minor_units_to_decimal(amount_to_minor_units(amount, scale), scale)
    subject = order.ai_agent_id
    context = {"amount": amount, "currency": order.currency_id.name,
               "resource.creator_id": "user:" + order.create_uid.login,
               "resource.department": order.pdp_department,
               "tool_context": "tool:auto_confirm_po", "execution_mode": "autonomous_run"}
    token = issue_jwt_from_environment(subject, order._tenant_id())
    decision, obligations, _ = client.check_access(
        order._tenant_id(), subject, "action:CONFIRM_PURCHASE_ORDER",
        "purchase_order:%s" % order.id, context, token)
    if decision != "ALLOW" or any(item["type"] != "REQUIRE_HUMAN_APPROVAL" for item in obligations):
        raise AccessError("Policy-only decision denied")
    if obligations:
        order.env.cr.execute("SELECT approver_id FROM eval_abc_approval WHERE order_id=%s", [order.id])
        if not order.env.cr.fetchone():
            order.write({"state": "to approve"})
            return "pending"
    native_confirm(order)
    return "allowed"


def execute(order, variant, outage=False):
    if order.env.cr.dbname != "odoo_eval_abc" or order.env.su:
        raise RuntimeError("Variants require isolated DB and non-superuser execution")
    if variant not in ("A", "B", "C"):
        raise ValueError("Unknown variant")
    client = SafePDPClient(target="127.0.0.1:1") if outage else get_pdp_client()
    if variant == "A":
        native_confirm(order)
        return "native"
    if variant == "B":
        return policy_only(order, client)
    # Real connection failure, not a mocked decision. No patch on a normal C run.
    boundary = patch.object(purchase_order, "get_pdp_client", return_value=client) if outage else nullcontext()
    with boundary:
        order.button_confirm()
    return "protected"


def approve(order, variant, approver_id):
    approver = order.env["res.users"].browse(approver_id)
    if variant == "C":
        request = order.env["pdp.approval.request"].sudo().search([("purchase_order_id", "=", order.id)])
        return request.with_user(approver).action_issue_capability()
    if (not approver.has_group("purchase.group_purchase_manager")
            or approver.company_id != order.company_id
            or approver in (order.create_uid, order.delegated_by_id)):
        raise AccessError("Ordinary approval failed SoD/role/company checks")
    subject = "user:" + approver.login
    token = issue_jwt_from_environment(subject, order._tenant_id(),
                                       attributes={"department": approver.pdp_department})
    decision, obligations, _ = get_pdp_client().check_access(
        order._tenant_id(), subject, "action:APPROVE_PURCHASE_ORDER", "purchase_order:%s" % order.id,
        {"approval_state": "pending", "resource.department": order.pdp_department}, token)
    if decision != "ALLOW" or obligations:
        raise AccessError("Ordinary approver policy denied")
    order.env.cr.execute("INSERT INTO eval_abc_approval (order_id, approver_id) VALUES (%s,%s)",
                         [order.id, approver.id])
    return True
