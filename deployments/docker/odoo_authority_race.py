"""Live PDP revocation ordered against an Odoo final transaction, with DB oracles."""

import threading
import time
import traceback
from unittest.mock import patch

import odoo.service.model
from odoo import SUPERUSER_ID, api
from odoo.exceptions import AccessError

from odoo_material_race import DEADLINE_SECONDS, _snapshot, _wait


def _observe_revoke_block(registry, final_pid, errors):
    deadline = time.monotonic() + DEADLINE_SECONDS
    with registry.cursor() as cursor:
        while time.monotonic() < deadline:
            if errors:
                raise AssertionError("authority worker failed: %r" % errors)
            cursor.execute("""
                SELECT pid FROM pg_stat_activity
                WHERE datname = current_database()
                  AND application_name = 'pdp-erp-revocation-fence'
                  AND %s = ANY(pg_blocking_pids(pid))
            """, [final_pid])
            if cursor.fetchone():
                return
            threading.Event().wait(0.02)
    raise AssertionError("PDP revoke did not block behind final ERP transaction")


def _run(registry, order_id, revoke_first=False, rollback=False):
    from odoo.addons.pdp_authorizer.models import delegation_grant, pdp_client

    before = _snapshot(registry, order_id)
    paused, release = threading.Event(), threading.Event()
    done = {role: threading.Event() for role in ("final", "revoke")}
    errors, pids, results = [], {}, {}
    calls = [0]
    model_class = registry["purchase.order"]
    boundary = "_lock_delegation_revocation_fence" if revoke_first else "button_approve"
    original = getattr(model_class, boundary)

    def pause_final(record, *args, **kwargs):
        if record.ids == [order_id]:
            paused.set()
            _wait(release, "resume after final live PDP ALLOW")
        value = original(record, *args, **kwargs)
        if rollback and record.ids == [order_id]:
            raise AccessError("authority-race injected rollback")
        return value

    def worker(role):
        try:
            with registry.cursor() as cursor:
                env = api.Environment(cursor, SUPERUSER_ID, {})
                cursor.execute("SELECT pg_backend_pid(), current_setting('transaction_isolation')")
                pids[role], isolation = cursor.fetchone()
                if isolation != "repeatable read":
                    raise AssertionError("unexpected Odoo isolation")
                order = env["purchase.order"].browse(order_id)
                if role == "revoke":
                    order.delegation_grant_id.action_revoke()
                    cursor.commit()
                    results[role] = True
                else:
                    def confirm():
                        calls[0] += 1
                        return order.button_confirm()
                    results[role] = odoo.service.model.retrying(confirm, env)
        except AccessError as exc:
            if role == "final" and rollback and str(exc) == "authority-race injected rollback":
                results[role] = "rolled_back"
            else:
                errors.append((role, traceback.format_exc()))
        except Exception:
            errors.append((role, traceback.format_exc()))
        finally:
            done[role].set()

    threads = {role: threading.Thread(target=worker, args=(role,), daemon=True) for role in done}
    started = []
    # Only give the real revoke RPC more time for the deliberate lock hold.
    # No authorization result is mocked; the production client default stays unchanged.
    revoke_client = pdp_client.SafePDPClient(timeout=DEADLINE_SECONDS)
    with patch.object(model_class, boundary, pause_final), patch.object(
        delegation_grant, "get_pdp_client", return_value=revoke_client
    ):
        try:
            threads["final"].start()
            started.append("final")
            _wait(paused, "final PDP ALLOW checkpoint")
            threads["revoke"].start()
            started.append("revoke")
            if revoke_first:
                _wait(done["revoke"], "revoke commits before fence acquisition")
                intermediate = _snapshot(registry, order_id)
                if intermediate["grant"] != "revoked" or intermediate["state"] != "to approve":
                    raise AssertionError("revoke-first checkpoint is not committed")
            else:
                _observe_revoke_block(registry, pids["final"], errors)
                if done["revoke"].is_set():
                    raise AssertionError("revoke reported success before final transaction released its lock")
            release.set()
            for role in done:
                _wait(done[role], role + " completion")
        finally:
            release.set()
            for role in started:
                threads[role].join(timeout=DEADLINE_SECONDS + 2)
            revoke_client._channel.close()
    if errors or any(threads[role].is_alive() for role in started):
        raise AssertionError("authority schedule failed: %r" % errors)
    actual = _snapshot(registry, order_id)
    for key in ("line_id", "name", "price_unit", "amount_total", "binding", "command"):
        if actual[key] != before[key]:
            raise AssertionError("authority race changed business intent: " + key)
    if actual["grant"] != "revoked" or results["revoke"] is not True:
        raise AssertionError("revocation did not persist")
    states = actual["state"], actual["approval"], actual["attempt"]
    expected = (
        ("to approve", "invalidated", "approval_required") if revoke_first else
        ("to approve", "approved", "approval_required") if rollback else
        ("purchase", "consumed", "executed")
    )
    if states != expected or (actual["result"] == "purchase") != (not revoke_first and not rollback):
        raise AssertionError("unsafe committed authority outcome: %r" % actual)
    if results["final"] != ("rolled_back" if rollback else True):
        raise AssertionError("unexpected final operation result")
    if revoke_first and (actual["reason"] != "delegation_revoked" or calls[0] < 2):
        raise AssertionError("stale snapshot was not retried and invalidated")
    with registry.cursor() as cursor:
        cursor.execute("SELECT revoked FROM pdp_delegation_fence_v1 WHERE grant_id = "
                       "(SELECT delegation_grant_id::text FROM purchase_order WHERE id = %s)", [order_id])
        if cursor.fetchall() != [(True,)]:
            raise AssertionError("committed revocation tombstone missing")
    print("ODOO-AUTHORITY-RACE PASS: %s; fresh-session states=%r; final attempts=%d" % (
        "revoke-first after ALLOW" if revoke_first else "final rollback then revoke" if rollback
        else "final commit then revoke (observed DB blocking)", states, calls[0]), flush=True)


def run_authority_races(registry, create_command):
    for revoke_first, rollback in ((True, False), (False, False), (False, True)):
        _run(registry, create_command(registry, approved=True), revoke_first, rollback)
