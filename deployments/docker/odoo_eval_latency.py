"""EVAL-03: committed protected-route timings on the existing isolated ABC DB."""
import argparse
from datetime import datetime, timezone
import json
import os
from pathlib import Path
import platform
import time

import odoo
from odoo import api

from eval_latency_metrics import Recorder, distribution, summarize
from odoo_abc_compare import snapshot_inputs
from odoo_abc_fixture import create_order, initialize, observe, registry, transact
from odoo_latency_hooks import instrument


def prepare(reg, ids, route):
    oid = create_order(reg, ids, 1000 if route == "direct" else 2500)
    if route == "approved":
        transact(reg, ids, oid, lambda order: order.button_confirm())
        pending = observe(reg, oid)
        if pending["state"] != "to approve" or pending["capabilities"] != ["pending"]:
            raise AssertionError("Pending preparation did not persist")

        def approve(order):
            request = order.env["pdp.approval.request"].sudo().search([
                ("purchase_order_id", "=", order.id)])
            request.with_user(ids["pdp_e2e_approver"]).action_issue_capability()
        transact(reg, ids, oid, approve)
        if observe(reg, oid)["capabilities"] != ["approved"]:
            raise AssertionError("Approval preparation did not persist")
    return oid


def measure(reg, ids, route, phase, index):
    row = {"route": route, "phase": phase, "index": index, "status": "FAIL",
           "events": [], "error_type": None, "commit_returned": False}
    recorder = Recorder()
    try:
        oid = prepare(reg, ids, route)
        row["before"] = observe(reg, oid)
        # Cursor acquisition, Environment construction, setup and human approval
        # are outside the total. No service.model.retrying or implicit retry.
        with reg.cursor() as cr:
            env = api.Environment(cr, ids["abc_service"], {})
            order = env["purchase.order"].browse(oid)
            if env.su or cr.dbname != "odoo_eval_abc":
                raise RuntimeError("Timing requires the isolated non-superuser route")
            with instrument(recorder), recorder.span("confirm_through_commit"):
                with recorder.span("protected_button_confirm"):
                    order.button_confirm()
                with recorder.span("commit"):
                    cr.commit()
                row["commit_returned"] = True
        row["after"] = observe(reg, oid)
        after = row["after"]
        if after["state"] != "purchase" or after["attempts"] != ["executed"]:
            raise AssertionError("No persistent protected final result")
        expected = ["consumed"] if route == "approved" else []
        if after["capabilities"] != expected:
            raise AssertionError("Unexpected capability consumption")
        if any(after[key] != row["before"][key] for key in ("amount", "lines", "vendor_id", "currency")):
            raise AssertionError("Business input changed during measurement")
        names = [event["boundary"] for event in recorder.events]
        for required in ("locked_cbi", "python_proof_sign", "rpc_check_access", "native_button_approve"):
            if required not in names:
                raise AssertionError("Missing instrumentation: " + required)
        if names.count("native_button_approve") != 1:
            raise AssertionError("Expected exactly one native final mutation call")
        if route == "approved" and "rpc_verify_capability" not in names:
            raise AssertionError("Approved route did not verify capability")
        row["status"] = "PASS"
    except Exception as error:
        row["error_type"] = type(error).__name__
        # No proof/token or arbitrary exception text is exported.
        if "oid" in locals():
            try:
                row["after"] = observe(reg, oid)
            except Exception as observer_error:
                row["observer_error_type"] = type(observer_error).__name__
    row["events"] = recorder.events
    return row


def warm_transport():
    from odoo.addons.pdp_authorizer.models.pdp_client import get_pdp_client
    from odoo.addons.pdp_authorizer.pdp_protocol import issue_jwt_from_environment
    client = get_pdp_client()
    if client._channel_credentials() is None:
        raise RuntimeError("EVAL-03 requires mTLS")
    tenant, agent = os.environ["PDP_TENANT_ID"], "agent:procurement_copilot"
    result = client.check_access(
        tenant, agent, "action:CONFIRM_PURCHASE_ORDER", "purchase_order:eval03-warmup",
        {"amount": "1000", "tool_context": "tool:auto_confirm_po", "execution_mode": "autonomous_run",
         "erp.revocation_fence": "odoo-revocation.v1:odoo_eval_abc"},
        issue_jwt_from_environment(agent, tenant))
    if result[0] != "ALLOW" or result[1]:
        raise AssertionError("Warm-up policy differs from the required fixture")
    return result[2]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--samples", type=int, default=200)
    parser.add_argument("--warmup", type=int, default=20)
    parser.add_argument("--smoke", action="store_true", help="Never label these samples final evidence")
    args = parser.parse_args()
    if args.samples < 1 or args.warmup < 1:
        parser.error("Positive samples and warm-up required")
    stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
    output = Path(os.environ["PDP_EVAL_OUTPUT_DIR"]) / ("V2_EVAL_03_ODOO_" + stamp + ".json")
    result = {"schema": "eval03.v1", "status": "RUNNING", "started_utc": stamp,
              "kind": "smoke" if args.smoke else "measurement", "rows": [],
              "planned_per_route": {"warmup": args.warmup, "sample": args.samples},
              "method": "sequential alternating direct/approved; inclusive spans; no retries; no timing subtraction",
              "source_sha256": snapshot_inputs(),
              "environment": {"odoo": odoo.release.version, "python": platform.python_version(),
                              "platform": platform.platform(), "cpu_count": os.cpu_count(),
                              "git_commit": os.environ.get("PDP_GIT_COMMIT", "unknown"),
                              "timer": vars(time.get_clock_info("perf_counter"))}}
    try:
        reg = registry()
        ids = initialize(reg)
        result["setup_policy_advice"] = warm_transport()
        with reg.cursor() as cr:
            cr.execute("SELECT version(), current_setting('transaction_isolation'), "
                       "current_setting('synchronous_commit'), current_database()")
            result["environment"]["postgres_settings"] = cr.fetchone()
        calibration = Recorder()
        noop = calibration.wrap("empty_wrapper", lambda: None)
        for _ in range(1000):
            noop()
        result["timer_calibration_raw_ns"] = [event["elapsed_ns"] for event in calibration.events]
        result["timer_calibration"] = distribution(result["timer_calibration_raw_ns"])
        for phase, count in (("warmup", args.warmup), ("sample", args.samples)):
            for index in range(count):
                routes = ("direct", "approved") if index % 2 == 0 else ("approved", "direct")
                for route in routes:
                    row = measure(reg, ids, route, phase, index)
                    result["rows"].append(row)
                    if row["status"] != "PASS":
                        raise RuntimeError("Measurement failed; retain diagnostic artifact and diagnose")
                if (index + 1) % 20 == 0:
                    print("EVAL03 %s %s/%s per route" % (phase, index + 1, count), flush=True)
        result["status"] = "PASS"
    except Exception as error:
        result["status"], result["error_type"] = "FAIL", type(error).__name__
        raise
    finally:
        result["finished_utc"] = datetime.now(timezone.utc).isoformat()
        result["summary"] = summarize(result["rows"])
        with output.open("x", encoding="utf-8") as stream:
            json.dump(result, stream, indent=2)
            stream.write("\n")
        print("EVAL03_RESULT=" + str(output), flush=True)


if __name__ == "__main__":
    main()
