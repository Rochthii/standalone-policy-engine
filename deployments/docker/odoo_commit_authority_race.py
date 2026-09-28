"""Post-ALLOW policy/role schedules and real-clock deferred expiry rollback."""

import threading
import time
import traceback
import uuid
from datetime import timedelta
from unittest.mock import patch

import odoo.service.model
from odoo import SUPERUSER_ID, api, fields
from odoo.exceptions import AccessError
from psycopg2.errors import CheckViolation

from odoo_authority_changes import _publish_policy, _wait_policy
from odoo_material_race import DEADLINE_SECONDS, _observe_block, _snapshot, _wait


def _run_change(registry, create_command, kind, writer_first):
    order_id = create_command(registry, approved=True)
    before = _snapshot(registry, order_id)
    policy_id = str(uuid.uuid4())
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        approval = env["pdp.approval.request"].search([("purchase_order_id", "=", order_id)])
        approver_id, group_id = approval.approver_user_id.id, env.ref("purchase.group_purchase_manager").id
    paused, release, connected = threading.Event(), threading.Event(), threading.Event()
    done = {role: threading.Event() for role in ("final", "writer")}
    errors, pids = [], {}
    model = registry["purchase.order"]
    boundary = "_lock_current_authority" if writer_first else "button_approve"
    original = getattr(model, boundary)
    calls = [0]

    def pause(record, *args, **kwargs):
        if record.ids == [order_id] and not paused.is_set():
            paused.set()
            _wait(release, "authority final release")
        return original(record, *args, **kwargs)

    def publisher_connected(pid):
        pids["writer"] = pid
        connected.set()

    def worker(role):
        try:
            if role == "writer" and kind == "policy":
                _publish_policy(policy_id, order_id, on_connected=publisher_connected)
            else:
                with registry.cursor() as cursor:
                    env = api.Environment(cursor, SUPERUSER_ID, {})
                    cursor.execute("SELECT pg_backend_pid()")
                    pids[role] = cursor.fetchone()[0]
                    if role == "writer":
                        connected.set()
                        env["res.users"].browse(approver_id).write({"groups_id": [(3, group_id)]})
                        cursor.commit()
                    else:
                        def confirm():
                            calls[0] += 1
                            return env["purchase.order"].browse(order_id).button_confirm()
                        odoo.service.model.retrying(confirm, env)
        except AccessError:
            if not (role == "final" and writer_first and kind == "policy"):
                errors.append((role, traceback.format_exc()))
        except Exception:
            errors.append((role, traceback.format_exc()))
        finally:
            done[role].set()

    threads = {role: threading.Thread(target=worker, args=(role,), daemon=True) for role in done}
    started = []
    try:
        with patch.object(model, boundary, pause):
            try:
                threads["final"].start()
                started.append("final")
                _wait(paused, "after actual final PDP ALLOW")
                threads["writer"].start()
                started.append("writer")
                _wait(connected, "authority writer connected")
                if writer_first:
                    _wait(done["writer"], "authority commits before final fence")
                else:
                    _observe_block(registry, pids["writer"], pids["final"], errors)
                    if done["writer"].is_set():
                        raise AssertionError("authority writer bypassed held final fence")
                release.set()
                for role in done:
                    _wait(done[role], role + " completion")
            finally:
                release.set()
                for role in started:
                    threads[role].join(timeout=DEADLINE_SECONDS + 2)
        if errors or any(threads[role].is_alive() for role in started):
            raise AssertionError("commit authority workers failed: %r" % errors)
        actual = _snapshot(registry, order_id)
        for key in ("lines", "amount_total", "binding", "command"):
            if actual[key] != before[key]:
                raise AssertionError("authority schedule changed business binding")
        expected = (("to approve", "invalidated" if kind == "role" else "approved", "approval_required")
                    if writer_first else ("purchase", "consumed", "executed"))
        if (actual["state"], actual["approval"], actual["attempt"]) != expected:
            raise AssertionError("unsafe authority outcome: %r" % actual)
        if (actual["result"] == "purchase") == writer_first:
            raise AssertionError("false execution receipt")
        if writer_first and calls[0] < 2:
            raise AssertionError("stale RR authority snapshot did not retry")
        print("ODOO-COMMIT-AUTHORITY PASS: %s %s; attempts=%d; fresh states=%r" % (
            kind, "writer-first after ALLOW" if writer_first else "final-first observed blocking",
            calls[0], expected), flush=True)
    finally:
        if kind == "policy":
            _publish_policy(policy_id, order_id, delete=True)
            _wait_policy(registry, order_id, "ALLOW")
        else:
            with registry.cursor() as cursor:
                env = api.Environment(cursor, SUPERUSER_ID, {})
                env["res.users"].browse(approver_id).write({"groups_id": [(4, group_id)]})
                cursor.commit()


def _run_expiry(registry, create_command, approved):
    order_id = create_command(registry, approved=approved)
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        order.delegation_grant_id.valid_until = fields.Datetime.now() + timedelta(seconds=3)
        cursor.commit()
    # The real decision passes, then real wall time crosses the grant deadline
    # after business mutation but before COMMIT. No mocked decision or clock.
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        order.button_confirm()
        if order.state != "purchase":
            raise AssertionError("expiry fixture never passed final live authorization")
        deadline = order.delegation_grant_id.valid_until
        env.flush_all()
        while fields.Datetime.now() <= deadline:
            time.sleep(0.05)
        try:
            cursor.commit()
        except CheckViolation as exc:
            if "Protected authority expired" not in str(exc):
                raise
            cursor.rollback()
        else:
            raise AssertionError("expired authority committed")
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        attempts = env["pdp.authorization.attempt"].search([("business_res_id", "=", order_id)])
        approvals = env["pdp.approval.request"].search([("purchase_order_id", "=", order_id)])
        if order.state != ("to approve" if approved else "draft"):
            raise AssertionError("expiry rollback leaked final PO")
        if any(a.state == "executed" for a in attempts) or any(a.state == "consumed" for a in approvals):
            raise AssertionError("expiry rollback leaked consumption")
    print("ODOO-COMMIT-EXPIRY PASS: %s real-clock deferred failure; fresh non-final/unconsumed" % (
        "approved" if approved else "direct"), flush=True)


def run_commit_authority_races(registry, create_command):
    for kind in ("policy", "role"):
        for writer_first in (True, False):
            _run_change(registry, create_command, kind, writer_first)
    for approved in (False, True):
        _run_expiry(registry, create_command, approved)
