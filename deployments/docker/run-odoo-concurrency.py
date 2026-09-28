"""Verify two Odoo database sessions serialize one delegated command."""

import os
import threading
import uuid
from datetime import timedelta

import odoo
import odoo.service.model
from odoo import Command, SUPERUSER_ID, api, fields
from odoo.tools import config

from odoo_material_race import run_material_races
from odoo_authority_race import run_authority_races
from odoo_authority_changes import run_committed_authority_changes
from odoo_commit_authority_race import run_commit_authority_races

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


def create_command(registry, approved=False):
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
        if approved:
            manager = env.ref("purchase.group_purchase_manager")
            approver = env["res.users"].search(
                [("login", "=", "pdp_e2e_approver")], limit=1
            )
            if not approver:
                approver = env["res.users"].with_context(no_reset_password=True).create(
                    {
                        "name": "PDP final concurrency approver",
                        "login": "pdp_e2e_approver",
                        "email": "pdp-final-approver-%s@example.test" % suffix,
                        "company_id": env.company.id,
                        "company_ids": [Command.set([env.company.id])],
                        "pdp_department": "Procurement",
                        "groups_id": [Command.link(manager.id)],
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
                            "price_unit": 2500 if approved else 1000,
                            "date_planned": fields.Datetime.now(),
                        }
                    )
                ],
            }
        )
        if approved:
            if not order.button_confirm() or order.state != "to approve":
                raise AssertionError("high-value command did not enter pending approval")
            approval = env["pdp.approval.request"].search(
                [("purchase_order_id", "=", order.id)]
            ).ensure_one()
            if not approval.with_user(approver).action_issue_capability():
                raise AssertionError("independent manager did not issue approval")
            if approval.state != "approved":
                raise AssertionError("approval was not committed as approved")
        order_id = order.id
        cursor.commit()
        return order_id


def run_workers(registry, order_id, approved=False):
    barrier = threading.Barrier(2)
    errors = []
    outcomes = []
    result_lock = threading.Lock()
    worker_state = threading.local()
    model_class = registry["purchase.order"]
    # Synchronize before the PO row lock; a barrier inside final intent would
    # deadlock because the first session already holds that same PO lock.
    boundary = "_locked_delegation_nonce"
    original_lock = getattr(model_class, boundary)

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

    setattr(model_class, boundary, synchronized_lock)
    try:
        workers = [threading.Thread(target=confirm, daemon=True) for _ in range(2)]
        for worker in workers:
            worker.start()
        for worker in workers:
            worker.join(timeout=20)
        if any(worker.is_alive() for worker in workers):
            raise RuntimeError("concurrent Odoo confirms exceeded the 20-second deadline")
    finally:
        setattr(model_class, boundary, original_lock)

    if errors or outcomes != [True, True]:
        raise AssertionError("concurrent confirms failed: outcomes=%r errors=%r" % (outcomes, errors))


def assert_one_outcome(registry, order_id, approved=False):
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
        if approved:
            approvals = env["pdp.approval.request"].search(
                [("purchase_order_id", "=", order.id)]
            )
            if (
                len(approvals) != 1
                or approvals.state != "consumed"
                or approvals.terminal_reason != "executed"
                or approvals.command_id != order.pdp_delegation_nonce
                or attempts.result_state != "purchase"
            ):
                raise AssertionError("approved command did not produce one consumed final result")
            print(
                "ODOO-APPROVED-CONCURRENCY PASS: 2 sessions, 1 command, "
                "1 consumed approval, 1 executed attempt, state=purchase",
                flush=True,
            )
            return
        print(
            "ODOO-CONCURRENCY PASS: 2 sessions, 1 nonce, 1 executed attempt, state=purchase",
            flush=True,
        )


def assert_stale_approved_intent_rejected(registry, order_id):
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        approval = env["pdp.approval.request"].search(
            [("purchase_order_id", "=", order_id)]
        ).ensure_one()
        attempt = approval.authorization_attempt_id
        if (
            order.state != "to approve"
            or order.order_line.price_unit != 2600
            or approval.state != "invalidated"
            or approval.terminal_reason != "intent_changed"
            or attempt.state != "approval_required"
            or attempt.result_state == "purchase"
        ):
            raise AssertionError("cross-session business edit reached an unauthorized final outcome")
        print(
            "ODOO-APPROVED-STALE-INTENT PASS: cross-session line edit persisted; "
            "old approval invalidated without final PO effect",
            flush=True,
        )


if __name__ == "__main__":
    configure_odoo()
    database_registry = odoo.registry(DATABASE)
    protected_order_id = create_command(database_registry)
    run_workers(database_registry, protected_order_id)
    assert_one_outcome(database_registry, protected_order_id)
    approved_order_id = create_command(database_registry, approved=True)
    run_workers(database_registry, approved_order_id, approved=True)
    assert_one_outcome(database_registry, approved_order_id, approved=True)
    # The caller discarded the committed result; a fresh session retries the same command.
    order_class = database_registry["purchase.order"]
    original_approve = order_class.button_approve

    def unexpected_second_transition(record):
        raise AssertionError("lost-response retry repeated the PO business transition")

    order_class.button_approve = unexpected_second_transition
    try:
        with database_registry.cursor() as retry_cursor:
            retry_env = api.Environment(retry_cursor, SUPERUSER_ID, {})
            if not retry_env["purchase.order"].browse(approved_order_id).button_confirm():
                raise AssertionError("lost-response retry did not return the terminal result")
            retry_cursor.commit()
    finally:
        order_class.button_approve = original_approve
    assert_one_outcome(database_registry, approved_order_id, approved=True)
    print("ODOO-APPROVED-RETRY PASS: fresh-session retry reused terminal command", flush=True)
    stale_order_id = create_command(database_registry, approved=True)
    with database_registry.cursor() as edit_cursor:
        edit_env = api.Environment(edit_cursor, SUPERUSER_ID, {})
        edit_env["purchase.order"].browse(stale_order_id).order_line.write(
            {"price_unit": 2600}
        )
        edit_cursor.commit()
    with database_registry.cursor() as final_cursor:
        final_env = api.Environment(final_cursor, SUPERUSER_ID, {})
        if not final_env["purchase.order"].browse(stale_order_id).button_confirm():
            raise AssertionError("stale approved command did not return safely")
        final_cursor.commit()
    assert_stale_approved_intent_rejected(database_registry, stale_order_id)
    run_authority_races(database_registry, create_command)
    run_committed_authority_changes(database_registry, create_command)
    run_commit_authority_races(database_registry, create_command)
    run_material_races(database_registry, create_command)
