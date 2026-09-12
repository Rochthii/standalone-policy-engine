"""Verify two Odoo database sessions serialize one delegated command."""

import os
import threading
import uuid
from datetime import timedelta

import odoo
import odoo.service.model
from odoo import Command, SUPERUSER_ID, api, fields
from odoo.tools import config


DATABASE = "odoo_e2e"


def configure_odoo():
    config.parse_config(
        [
            "--database=" + DATABASE,
            "--db_host=" + os.environ["HOST"],
            "--db_port=" + os.environ.get("PDP_DB_PORT", "5432"),
            "--db_user=" + os.environ["USER"],
            "--db_password=" + os.environ["PASSWORD"],
            "--without-demo=all",
        ]
    )


def create_command(registry):
    suffix = uuid.uuid4().hex
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        env.company.write(
            {
                "pdp_tenant_id": os.environ["PDP_TENANT_ID"],
                "po_double_validation": "one_step",
            }
        )
        env.user.write({"pdp_department": "Procurement"})
        creator = env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "PDP concurrency creator",
                "login": "pdp_concurrency_" + suffix,
                "email": "pdp-concurrency-%s@example.test" % suffix,
                "company_id": env.company.id,
                "company_ids": [Command.set([env.company.id])],
                "pdp_department": "Procurement",
            }
        )
        vendor = env["res.partner"].create(
            {"name": "PDP concurrency vendor " + suffix, "supplier_rank": 1}
        )
        product = env["product.product"].create(
            {"name": "PDP concurrency product " + suffix, "purchase_ok": True}
        )
        now = fields.Datetime.now()
        grant = env["pdp.delegation.grant"].create(
            {
                "user_id": env.user.id,
                "agent_id": "agent:procurement_copilot",
                "max_amount": 2000,
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
            }
        )
        grant.action_activate()
        order = env["purchase.order"].with_user(creator).sudo().create(
            {
                "partner_id": vendor.id,
                "company_id": env.company.id,
                "pdp_department": "Procurement",
                "ai_agent_id": grant.agent_id,
                "delegated_by_id": grant.user_id.id,
                "delegation_grant_id": grant.id,
                "order_line": [
                    Command.create(
                        {
                            "name": product.display_name,
                            "product_id": product.id,
                            "product_qty": 1,
                            "product_uom": product.uom_po_id.id,
                            "price_unit": 1000,
                            "date_planned": fields.Datetime.now(),
                        }
                    )
                ],
            }
        )
        order_id = order.id
        cursor.commit()
        return order_id


def run_workers(registry, order_id):
    barrier = threading.Barrier(2)
    errors = []
    outcomes = []
    result_lock = threading.Lock()
    worker_state = threading.local()
    model_class = registry["purchase.order"]
    original_lock = model_class._locked_delegation_nonce

    def synchronized_lock(record):
        if not getattr(worker_state, "synchronized", False):
            worker_state.synchronized = True
            barrier.wait(timeout=10)
        return original_lock(record)

    def confirm():
        try:
            with registry.cursor() as cursor:
                env = api.Environment(cursor, SUPERUSER_ID, {})
                result = odoo.service.model.retrying(
                    lambda: env["purchase.order"].browse(order_id).button_confirm(),
                    env,
                )
                with result_lock:
                    outcomes.append(result)
        except Exception as exc:  # surfaced with full representation below
            with result_lock:
                errors.append(repr(exc))

    model_class._locked_delegation_nonce = synchronized_lock
    try:
        workers = [threading.Thread(target=confirm, daemon=True) for _ in range(2)]
        for worker in workers:
            worker.start()
        for worker in workers:
            worker.join(timeout=20)
        if any(worker.is_alive() for worker in workers):
            raise RuntimeError("concurrent Odoo confirms exceeded the 20-second deadline")
    finally:
        model_class._locked_delegation_nonce = original_lock

    if errors or outcomes != [True, True]:
        raise AssertionError("concurrent confirms failed: outcomes=%r errors=%r" % (outcomes, errors))


def assert_one_outcome(registry, order_id):
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        attempts = env["pdp.authorization.attempt"].search(
            [("business_model", "=", order._name), ("business_res_id", "=", order.id)]
        )
        if order.state != "purchase" or not order.pdp_delegation_nonce:
            raise AssertionError(
                "concurrent command did not reach one committed purchase outcome"
            )
        if len(attempts) != 1 or attempts.state != "executed":
            raise AssertionError(
                "expected one executed nonce attempt, got %d state=%s"
                % (len(attempts), attempts.mapped("state"))
            )
        print(
            "ODOO-CONCURRENCY PASS: 2 sessions, 1 nonce, 1 executed attempt, state=purchase",
            flush=True,
        )


if __name__ == "__main__":
    configure_odoo()
    database_registry = odoo.registry(DATABASE)
    protected_order_id = create_command(database_registry)
    run_workers(database_registry, protected_order_id)
    assert_one_outcome(database_registry, protected_order_id)
