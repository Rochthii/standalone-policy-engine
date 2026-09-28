"""ABC-only fixtures and fresh-transaction observations; never loaded by the addon."""
import os
import secrets
from datetime import timedelta

import odoo
from odoo import api, Command, fields, SUPERUSER_ID
from odoo.tools import config

DATABASE = "odoo_eval_abc"


def registry():
    if os.environ.get("PDP_EVAL_ONLY") != DATABASE:
        raise RuntimeError("Not the isolated ABC testbed")
    config.parse_config([
        "--database=" + DATABASE, "--db_host=" + os.environ["HOST"],
        "--db_user=" + os.environ["USER"], "--db_password=" + os.environ["PASSWORD"],
        "--without-demo=all",
    ])
    return odoo.registry(DATABASE)


def initialize(reg):
    with reg.cursor() as cr:
        env = api.Environment(cr, SUPERUSER_ID, {})
        env.company.write({"pdp_tenant_id": os.environ["PDP_TENANT_ID"],
                           "po_double_validation": "one_step"})
        env.user.write({"pdp_department": "Procurement"})
        ids = {}
        for name in ("abc_creator", "abc_service", "pdp_e2e_approver"):
            user = env["res.users"].search([("login", "=", name)], limit=1)
            if not user:
                user = env["res.users"].with_context(no_reset_password=True).create({
                    "name": name, "login": name, "email": name + "@example.test",
                    "company_id": env.company.id, "company_ids": [Command.set([env.company.id])],
                    "groups_id": [Command.link(env.ref("purchase.group_purchase_manager").id)],
                    "pdp_department": "Procurement",
                })
            ids[name] = user.id
        ids["vendor"] = env["res.partner"].create({"name": "ABC vendor", "supplier_rank": 1}).id
        ids["product"] = env["product.product"].create({
            "name": "ABC product", "purchase_ok": True, "supplier_taxes_id": [Command.clear()],
        }).id
        # B's ordinary per-PO approval: intentionally no hash, witness or consumption.
        cr.execute("CREATE TABLE IF NOT EXISTS eval_abc_approval "
                   "(order_id integer PRIMARY KEY, approver_id integer NOT NULL)")
        cr.commit()
        return ids


def create_order(reg, ids, amount, self_delegation=False, grant_id=None, nonce=None):
    with reg.cursor() as cr:
        env = api.Environment(cr, SUPERUSER_ID, {})
        creator = ids["abc_creator"]
        grant = env["pdp.delegation.grant"].browse(grant_id) if grant_id else None
        if not grant:
            now = fields.Datetime.now()
            grant = env["pdp.delegation.grant"].create({
                "user_id": creator if self_delegation else env.user.id,
                "agent_id": "agent:procurement_copilot", "max_amount": 2000,
                "valid_from": now - timedelta(minutes=1), "valid_until": now + timedelta(minutes=30),
            })
            grant.action_activate()
        product = env["product.product"].browse(ids["product"])
        order = env["purchase.order"].with_user(creator).create({
            "partner_id": ids["vendor"], "company_id": env.company.id,
            "pdp_department": "Procurement", "ai_agent_id": grant.agent_id,
            "delegated_by_id": grant.user_id.id, "delegation_grant_id": grant.id,
            "pdp_delegation_nonce": nonce or secrets.token_urlsafe(32),
            "order_line": [Command.create({
                "name": "ABC line", "product_id": product.id, "product_qty": 1,
                "product_uom": product.uom_po_id.id, "price_unit": amount,
                "taxes_id": [Command.clear()], "date_planned": fields.Datetime.now(),
            })],
        })
        result = order.id
        cr.commit()
        return result


def transact(reg, ids, order_id, operation):
    with reg.cursor() as cr:
        env = api.Environment(cr, ids["abc_service"], {})
        value = operation(env["purchase.order"].browse(order_id))
        cr.commit()
        return value


def observe(reg, order_id):
    with reg.cursor() as cr:
        env = api.Environment(cr, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        cr.execute("SELECT amount_total::text FROM purchase_order WHERE id=%s", [order_id])
        amount = cr.fetchone()[0]
        attempts = env["pdp.authorization.attempt"].search([
            ("business_model", "=", "purchase.order"), ("business_res_id", "=", order_id)])
        approvals = env["pdp.approval.request"].search([("purchase_order_id", "=", order_id)])
        cr.execute("SELECT approver_id FROM eval_abc_approval WHERE order_id=%s", [order_id])
        ordinary = cr.fetchone()
        return {"order_id": order_id, "state": order.state, "amount": amount,
                "vendor_id": order.partner_id.id, "currency": order.currency_id.name,
                "lines": order.order_line.mapped("name"), "grant_id": order.delegation_grant_id.id,
                "grant_state": order.delegation_grant_id.state, "nonce": order.pdp_delegation_nonce,
                "attempts": attempts.mapped("state"), "capabilities": approvals.mapped("state"),
                "ordinary_approval": ordinary[0] if ordinary else None}
