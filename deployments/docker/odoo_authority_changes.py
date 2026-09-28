"""Committed authority changes before final execution; not commit-time atomicity."""

import os
import time
import uuid

import psycopg2
from odoo import SUPERUSER_ID, api, fields
from odoo.exceptions import AccessError

from odoo_material_race import _snapshot


def _publish_policy(policy_id, order_id, delete=False, on_connected=None):
    """Fixture publisher follows the durable barrier used by Go policy writers."""
    connection = psycopg2.connect(host=os.environ["HOST"], user=os.environ["USER"],
        password=os.environ["PASSWORD"], dbname="odoo_e2e")
    tenant, token = os.environ["PDP_TENANT_ID"], str(uuid.uuid4())
    try:
        if on_connected:
            on_connected(connection.get_backend_pid())
        with connection:
            with connection.cursor() as cursor:
                cursor.execute("""UPDATE pdp_policy_fence_v1 SET ready=false, publication_id=%s
                    WHERE tenant_id=%s AND ready=true""", [token, tenant])
                if cursor.rowcount != 1:
                    raise AssertionError("fixture policy publication is already pending")
        revision = _publish_policy_storage(policy_id, order_id, delete)
        with connection:
            with connection.cursor() as cursor:
                cursor.execute("""UPDATE pdp_policy_fence_v1 SET ready=true, revision=%s,
                    publication_id=NULL WHERE tenant_id=%s AND publication_id=%s AND ready=false""",
                    [revision, tenant, token])
                if cursor.rowcount != 1:
                    raise AssertionError("fixture lost policy publication ownership")
        return revision
    finally:
        connection.close()


def _publish_policy_storage(policy_id, order_id, delete=False):
    connection = psycopg2.connect(host=os.environ["HOST"], user=os.environ["USER"],
                                 password=os.environ["PASSWORD"], dbname="policy_engine")
    try:
        with connection:
            with connection.cursor() as cursor:
                tenant = os.environ["PDP_TENANT_ID"]
                if delete:
                    cursor.execute("DELETE FROM policies WHERE id = %s AND tenant_id = %s", [policy_id, tenant])
                else:
                    source = ("forbid(principal == agent:procurement_copilot, "
                              "action == action:CONFIRM_PURCHASE_ORDER, "
                              'resource == purchase_order:"%d");' % order_id)
                    cursor.execute("""INSERT INTO policies
                        (id, tenant_id, effect, policy_text, status, version)
                        VALUES (%s, %s, 'FORBID', %s, 'ACTIVE', 1)""", [policy_id, tenant, source])
                cursor.execute("UPDATE tenants SET revision = revision + 1 WHERE id = %s RETURNING revision", [tenant])
                revision = cursor.fetchone()[0]
                cursor.execute("""SELECT pg_notify('policy_events', json_build_object(
                    'tenant_id', %s::text, 'policy_id', %s::text,
                    'action', %s::text, 'revision', %s)::text)""",
                    [tenant, policy_id, "DELETE" if delete else "UPDATE", revision])
                return revision
    finally:
        connection.close()


def _wait_policy(registry, order_id, expected):
    from odoo.addons.pdp_authorizer.models.pdp_client import get_pdp_client
    from odoo.addons.pdp_authorizer.pdp_protocol import issue_jwt_from_environment

    deadline = time.monotonic() + 15
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        order = env["purchase.order"].browse(order_id)
        intent, proof, _, issued, expires = order.delegation_grant_id.build_protected_intent(
            order, order.pdp_delegation_nonce)
        request = order._protected_delegated_request(intent, proof, issued, expires)
        token = issue_jwt_from_environment(intent["agent_subject"], intent["tenant_id"])
        while time.monotonic() < deadline:
            decision, _, _ = get_pdp_client().check_access(intent["tenant_id"], request["subject"],
                request["action"], request["resource"], request["context"], token)
            if decision == expected:
                return
            time.sleep(0.05)
    raise AssertionError("live PDP did not observe committed policy: " + expected)


def run_committed_authority_changes(registry, create_command):
    from datetime import timedelta

    for kind in ("policy", "role", "expiry"):
        order_id = create_command(registry, approved=True)
        before = _snapshot(registry, order_id)
        policy_id = str(uuid.uuid4())
        approver_id = group_id = None
        revision = None
        try:
            if kind == "policy":
                _wait_policy(registry, order_id, "ALLOW")
                revision = _publish_policy(policy_id, order_id)
                _wait_policy(registry, order_id, "DENY")
            else:
                with registry.cursor() as cursor:
                    env = api.Environment(cursor, SUPERUSER_ID, {})
                    order = env["purchase.order"].browse(order_id)
                    if kind == "expiry":
                        order.delegation_grant_id.valid_until = fields.Datetime.now() - timedelta(seconds=1)
                    else:
                        approval = env["pdp.approval.request"].search([("purchase_order_id", "=", order_id)]).ensure_one()
                        approver_id = approval.approver_user_id.id
                        group_id = env.ref("purchase.group_purchase_manager").id
                        approval.approver_user_id.write({"groups_id": [(3, group_id)]})
                    cursor.commit()
            with registry.cursor() as cursor:
                env = api.Environment(cursor, SUPERUSER_ID, {})
                if kind == "policy":
                    try:
                        with cursor.savepoint():
                            env["purchase.order"].browse(order_id).button_confirm()
                    except AccessError:
                        pass
                    else:
                        raise AssertionError("live denying policy was ignored")
                elif not env["purchase.order"].browse(order_id).button_confirm():
                    raise AssertionError("authority invalidation did not return safely")
                cursor.commit()
            actual = _snapshot(registry, order_id)
            for key in ("lines", "amount_total", "binding", "command"):
                if actual[key] != before[key]:
                    raise AssertionError("authority change altered intent/consumption binding")
            expected = ("to approve", "approved" if kind == "policy" else "invalidated", "approval_required")
            if (actual["state"], actual["approval"], actual["attempt"]) != expected or actual["result"] == "purchase":
                raise AssertionError("stale authority reached final mutation: %r" % actual)
            print("ODOO-AUTHORITY-CHANGE PASS: %s committed before final; revision=%r; fresh-session non-final/unconsumed" % (kind, revision), flush=True)
        finally:
            if kind == "policy":
                _publish_policy(policy_id, order_id, delete=True)
                _wait_policy(registry, order_id, "ALLOW")
            elif approver_id:
                with registry.cursor() as cursor:
                    env = api.Environment(cursor, SUPERUSER_ID, {})
                    env["res.users"].browse(approver_id).write({"groups_id": [(4, group_id)]})
                    cursor.commit()
