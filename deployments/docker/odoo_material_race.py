"""Controlled overlapping ORM edits and approved final execution on real sessions."""

import threading
import time
import traceback
from unittest.mock import patch

import odoo.service.model
from odoo import SUPERUSER_ID, api
from odoo.exceptions import UserError


DEADLINE_SECONDS = 15


def _business(order):
    order.invalidate_recordset()
    lines = order.order_line.sorted("id")
    lines.invalidate_recordset()
    first = lines[:1]
    return {
        "line_id": first.id if first else False,
        "name": first.name if first else False,
        "price_unit": first.price_unit if first else False,
        "amount_total": order.amount_total,
        "lines": tuple((line.id, line.name, line.price_unit, line.product_qty,
                        tuple(sorted(line.taxes_id.ids))) for line in lines),
        "vendor_id": order.partner_id.id,
        "currency_id": order.currency_id.id,
    }


def _wait(event, label):
    if not event.wait(DEADLINE_SECONDS):
        raise AssertionError("material race timed out: " + label)


def _snapshot(registry, order_id):
    # A new session observes committed data, independent of both worker caches.
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        approval = env["pdp.approval.request"].search(
            [("purchase_order_id", "=", order_id)]
        ).ensure_one()
        attempt = env["pdp.authorization.attempt"].search(
            [("business_model", "=", "purchase.order"), ("business_res_id", "=", order_id)]
        ).ensure_one()
        return {
            **_business(order),
            "state": order.state,
            "grant": approval.delegation_grant_id.state,
            "approval": approval.state,
            "reason": approval.terminal_reason,
            "attempt": attempt.state,
            "result": attempt.result_state,
            "binding": (approval.intent_hash, approval.one_time_id, approval.command_id),
            "command": (order.pdp_delegation_nonce, attempt.delegation_nonce),
        }


def _expect_outcome(actual, expected, final):
    keys = ("lines", "amount_total", "vendor_id", "currency_id", "binding", "command")
    if any(actual[key] != expected[key] for key in keys):
        raise AssertionError("material race changed unexpected data: %r" % actual)
    states = (actual["state"], actual["approval"], actual["attempt"], actual["reason"])
    required = (
        (expected.get("edited_state", "purchase"), "consumed", "executed", "executed")
        if final else (expected.get("edited_state", "to approve"), "invalidated", "approval_required", "intent_changed")
    )
    if states != required or (actual["result"] == "purchase") != final:
        raise AssertionError("material race violated commit outcome: %r" % actual)


def _observe_block(registry, waiter, holder, errors):
    deadline = time.monotonic() + DEADLINE_SECONDS
    with registry.cursor() as cursor:
        while time.monotonic() < deadline:
            if errors:
                raise AssertionError("material race worker failed: %r" % errors)
            cursor.execute("SELECT %s = ANY(pg_blocking_pids(%s))", [holder, waiter])
            if cursor.fetchone()[0]:
                return
            # Poll actual PostgreSQL lock ownership; elapsed time is not evidence.
            threading.Event().wait(0.05)
    raise AssertionError("expected PostgreSQL blocking was not observed")


def _run_race(registry, order_id, field, edit_first):
    before = _snapshot(registry, order_id)
    expected = dict(before)
    release_edit = threading.Event()
    release_final = threading.Event()
    edit_flushed = threading.Event()
    final_locked = threading.Event()
    ready = {role: threading.Event() for role in ("edit", "final")}
    done = {role: threading.Event() for role in ready}
    pids, outcomes, errors = {}, {}, []
    calls = {role: 0 for role in ready}
    result_lock = threading.Lock()
    model_class = registry["purchase.order"]
    original_approve = model_class.button_approve

    def pause_before_mutation(record, *args, **kwargs):
        if record.ids == [order_id] and not edit_first:
            final_locked.set()
            _wait(release_final, "release final mutation")
        return original_approve(record, *args, **kwargs)

    def worker(role):
        try:
            with registry.cursor() as cursor:
                env = api.Environment(cursor, SUPERUSER_ID, {})
                cursor.execute("SELECT pg_backend_pid(), current_setting('transaction_isolation')")
                backend_pid, isolation = cursor.fetchone()
                if isolation != "repeatable read":
                    raise AssertionError("unexpected Odoo transaction isolation: " + isolation)
                with result_lock:
                    pids[role] = backend_pid
                ready[role].set()

                def operation():
                    calls[role] += 1
                    cursor.execute("SET LOCAL statement_timeout = '15000ms'")
                    if role == "final":
                        return env["purchase.order"].browse(order_id).button_confirm()
                    order = env["purchase.order"].browse(order_id)
                    line = env["purchase.order.line"].browse(before["line_id"])
                    if field == "add_line":
                        env["purchase.order.line"].create({"order_id": order_id,
                            "display_type": "line_note", "product_qty": 0,
                            "name": "Concurrent new material line"})
                    elif field == "delete_line":
                        line.unlink()
                    elif field == "taxes_id":
                        tax = env["account.tax"].create({"name": "Concurrent zero tax %s" % order_id,
                            "amount": 0, "type_tax_use": "purchase", "company_id": order.company_id.id})
                        line.write({"taxes_id": [(6, 0, tax.ids)]})
                    elif field == "partner_id":
                        vendor = env["res.partner"].create({"name": "Concurrent vendor", "supplier_rank": 1})
                        order.write({"partner_id": vendor.id})
                    elif field == "currency_id":
                        currency = env["res.currency"].with_context(active_test=False).search(
                            [("id", "!=", before["currency_id"])], limit=1)
                        currency.active = True
                        order.write({"currency_id": currency.id})
                    elif field == "state":
                        order.write({"state": "sent"})
                        expected["edited_state"] = "sent"
                    else:
                        value = "Concurrent approved-line edit" if field == "name" else 2600
                        line.write({field: value})
                    env.flush_all()
                    expected.update(_business(order))
                    if expected == before:
                        raise AssertionError("material fixture did not change business data")
                    edit_flushed.set()
                    _wait(release_edit, "release edit commit")
                    return True

                result = odoo.service.model.retrying(operation, env)
                with result_lock:
                    outcomes[role] = result
        except UserError as exc:
            if (role == "edit" and field == "delete_line" and not edit_first
                    and str(exc).startswith("Cannot delete a purchase order line which is in state")):
                # Native Odoo rejects deleting a line once final execution won.
                outcomes[role] = "denied"
                edit_flushed.set()
            else:
                errors.append((role, traceback.format_exc()))
        except Exception:
            with result_lock:
                errors.append((role, traceback.format_exc()))
        finally:
            done[role].set()

    workers = {role: threading.Thread(target=worker, args=(role,), daemon=True) for role in ready}
    started = []
    with patch.object(model_class, "button_approve", pause_before_mutation):
        try:
            first, second = ("edit", "final") if edit_first else ("final", "edit")
            workers[first].start()
            started.append(first)
            _wait(edit_flushed if edit_first else final_locked, "first session holds locks")
            workers[second].start()
            started.append(second)
            _wait(ready[second], "second backend PID")
            _observe_block(registry, pids[second], pids[first], errors)
            if edit_first:
                release_edit.set()
                _wait(done["edit"], "edit committed")
                _wait(done["final"], "final revalidation completed")
                if calls["final"] < 2:
                    raise AssertionError("edit-first final session did not retry its stale snapshot")
                _expect_outcome(_snapshot(registry, order_id), expected, final=False)
            else:
                release_final.set()
                _wait(done["final"], "final committed")
                _wait(edit_flushed, "waiting edit resumed")
                # The writer still has not committed. Observe the exact approved
                # business data, consumed AC and executed command at final commit.
                _expect_outcome(_snapshot(registry, order_id), before, final=True)
                release_edit.set()
                _wait(done["edit"], "later edit committed")
                _expect_outcome(_snapshot(registry, order_id),
                                before if outcomes.get("edit") == "denied" else expected, final=True)
        finally:
            release_edit.set()
            release_final.set()
            for role in started:
                workers[role].join(timeout=DEADLINE_SECONDS + 2)
            if errors:
                print("ODOO-MATERIAL-RACE worker errors: %r" % errors, flush=True)
    if errors or any(workers[role].is_alive() for role in started) or outcomes.get("final") is not True or outcomes.get("edit") not in (True, "denied"):
        raise AssertionError("material race failed: outcomes=%r errors=%r" % (outcomes, errors))
    print(
        "ODOO-MATERIAL-RACE PASS: field=%s order=%s; observed DB blocking; "
        "fresh-session commit oracle; attempts=%r"
        % (field, "edit-first" if edit_first else "final-first", calls),
        flush=True,
    )


def run_material_races(registry, create_command):
    for field in ("name", "price_unit", "add_line", "delete_line", "taxes_id", "partner_id", "currency_id", "state"):
        for edit_first in (True, False):
            order_id = create_command(registry, approved=True)
            _run_race(registry, order_id, field, edit_first)
