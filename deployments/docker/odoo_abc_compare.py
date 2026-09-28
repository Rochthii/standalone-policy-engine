"""Run the same committed PO scenarios across A/B/C; emit raw observations."""
import hashlib
import json
import os
from pathlib import Path
import platform
from datetime import datetime, timezone

import odoo
from odoo.exceptions import AccessError, UserError

from odoo_abc_fixture import create_order, initialize, observe, registry, transact

CASES = ("direct_allow", "approval_required", "policy_deny", "approved", "stale_description",
         "stale_amount", "revoked_grant", "self_delegation", "same_record_retry",
         "cross_record_replay", "pdp_outage")


def snapshot_inputs():
    roots = [Path("/mnt/extra-addons/pdp_authorizer"), Path("/opt/pdp-clients"), Path("/opt/pdp-eval")]
    hashes = {}
    for root in roots:
        for path in sorted(root.rglob("*")):
            if path.suffix in (".py", ".csv", ".xml", ".sql") and "__pycache__" not in path.parts:
                hashes[str(path)] = hashlib.sha256(path.read_bytes()).hexdigest()
    return hashes


def attempt(reg, ids, oid, variant, outage=False):
    from odoo_abc_variants import execute
    try:
        result = transact(reg, ids, oid, lambda order: execute(order, variant, outage))
        return {"return": result, "error": None}
    except (AccessError, UserError) as error:
        # The transaction context rolls back; fresh observer is the real oracle.
        cause = error.__cause__
        return {"return": None, "error": type(error).__name__, "message": str(error),
                "transport_code": str(cause.code()) if callable(getattr(cause, "code", None)) else None}


def run_case(reg, ids, case, variant):
    from odoo_abc_variants import approve
    high = case in ("approval_required", "policy_deny", "approved", "stale_description", "stale_amount")
    oid = create_order(reg, ids, 2500 if high else 1000, case == "self_delegation")
    preparation = []
    if case == "policy_deny":
        transact(reg, ids, oid, lambda order: order.write({"pdp_department": "Blocked"}))
    if case in ("approved", "stale_description", "stale_amount"):
        if variant == "A":
            # Align review-time PO state without pretending A enforces approval.
            transact(reg, ids, oid, lambda order: order.write({"state": "to approve"}))
        else:
            preparation.append(attempt(reg, ids, oid, variant))
            if observe(reg, oid)["state"] != "to approve":
                raise AssertionError("Initial approval routing failed: %r" % preparation)
        transact(reg, ids, oid, lambda order: approve(order, variant, ids["pdp_e2e_approver"]))
        if case == "stale_description":
            transact(reg, ids, oid, lambda order: order.order_line.write({"name": "Changed after review"}))
        elif case == "stale_amount":
            transact(reg, ids, oid, lambda order: order.order_line.write({"price_unit": 2600}))
    if case == "revoked_grant":
        transact(reg, ids, oid, lambda order: order.delegation_grant_id.action_revoke())
    source = None
    if case in ("same_record_retry", "cross_record_replay"):
        preparation.append(attempt(reg, ids, oid, variant))
        source = observe(reg, oid)
        if source["state"] != "purchase":
            raise AssertionError("Replay setup failed: %r" % preparation)
        if case == "cross_record_replay":
            oid = create_order(reg, ids, 1000, grant_id=source["grant_id"], nonce=source["nonce"])
    before = observe(reg, oid)
    response = attempt(reg, ids, oid, variant, case == "pdp_outage")
    after = observe(reg, oid)
    return {"case": case, "variant": variant, "preparation": preparation,
            "source_order": source, "before": before, "response": response, "after": after}


def validate(row):
    case, variant, before, after = row["case"], row["variant"], row["before"], row["after"]
    if case in ("direct_allow", "approved", "same_record_retry"):
        assert after["state"] == "purchase", row
    if variant == "C":
        if case in ("direct_allow", "approved", "same_record_retry"):
            assert after["attempts"] == ["executed"], row
            if case == "approved":
                assert after["capabilities"] == ["consumed"], row
        else:
            assert after["state"] not in ("purchase", "done"), row
            assert "executed" not in after["attempts"] and "consumed" not in after["capabilities"], row
        if case in ("stale_description", "stale_amount"):
            assert after["capabilities"] == ["invalidated"], row
        if case == "cross_record_replay":
            assert "replayed" in row["response"].get("message", ""), row
    if variant == "B" and case in ("approval_required", "policy_deny", "pdp_outage"):
        assert after["state"] not in ("purchase", "done"), row
    if variant in ("B", "C") and case == "pdp_outage":
        assert row["response"].get("transport_code") == "StatusCode.UNAVAILABLE", row
    if variant in ("A", "B"):
        assert not after["attempts"] and not after["capabilities"], row
    assert after["amount"] == before["amount"] and after["lines"] == before["lines"], row
    if case == "same_record_retry":
        assert after == before, row
    # A/B weakness outcomes remain observed results, not assumed failure labels.


def main():
    reg = registry()
    ids = initialize(reg)
    from odoo.addons.pdp_authorizer.models.pdp_client import get_pdp_client
    from odoo.addons.pdp_authorizer.pdp_protocol import issue_jwt_from_environment
    tenant, agent = os.environ["PDP_TENANT_ID"], "agent:procurement_copilot"
    # Bootstrap the real PDP fence before any measured ERP transaction snapshot.
    # This read-only decision performs no business mutation and is not a case.
    warmup = get_pdp_client().check_access(
        tenant, agent, "action:CONFIRM_PURCHASE_ORDER", "purchase_order:abc-warmup",
        {"amount": "1000", "tool_context": "tool:auto_confirm_po", "execution_mode": "autonomous_run",
         "erp.revocation_fence": "odoo-revocation.v1:odoo_eval_abc"},
        issue_jwt_from_environment(agent, tenant))
    if warmup[0] != "ALLOW" or warmup[1]:
        raise AssertionError("PDP warm-up did not reach the common permit policy")
    with reg.cursor() as cr:
        cr.execute("SELECT version()")
        postgres = cr.fetchone()[0]
    timestamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    output = Path(os.environ["PDP_EVAL_OUTPUT_DIR"]) / ("V2_EVAL_02_ABC_" + timestamp + ".json")
    result = {"status": "RUNNING", "started_utc": timestamp, "cases": [],
              "setup_policy_advice": warmup[2],
              "environment": {"odoo": odoo.release.version, "postgres": postgres,
                              "python": platform.python_version(), "platform": platform.platform(),
                              "database": "odoo_eval_abc", "git_commit": os.environ.get("PDP_GIT_COMMIT", "unknown")},
              "source_sha256": snapshot_inputs(),
              "method": "11 scenarios x 3 variants; committed transactions and fresh ORM/PG observers; no latency claim"}
    try:
        for case in CASES:
            for variant in ("A", "B", "C"):
                row = run_case(reg, ids, case, variant)
                result["cases"].append(row)
                validate(row)
                print("ABC %s %s state=%s attempts=%s approvals=%s" % (
                    case, variant, row["after"]["state"], row["after"]["attempts"], row["after"]["capabilities"]), flush=True)
        result["status"] = "PASS"
    except Exception as error:
        result["status"], result["failure"] = "FAIL", repr(error)
        raise
    finally:
        output.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        print("ABC_RESULT=" + str(output), flush=True)


if __name__ == "__main__":
    main()
