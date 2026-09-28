"""Focused stdlib tests; no Docker, credentials or business mutation."""
import importlib.util
from copy import deepcopy
from pathlib import Path
import unittest

from eval_latency_metrics import Recorder, distribution, summarize

spec = importlib.util.spec_from_file_location("eval_summary", Path(__file__).with_name("summarize-eval-latency.py"))
summary_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(summary_module)


class MetricsTests(unittest.TestCase):
    def test_percentiles_and_empty(self):
        result = distribution([40, 10, 30, 20])
        for key, value in {"count": 4, "p50_ns": 25, "p95_ns": 38.5,
                           "p99_ns": 39.7, "max_ns": 40}.items():
            self.assertAlmostEqual(result[key], value)
        self.assertIsNone(distribution([])["p99_ns"])
        self.assertEqual(distribution([7])["p99_ns"], 7)
        for values in ([-1], [1.5], [True]):
            with self.assertRaises(ValueError):
                distribution(values)

    def test_wrapper_returns_original_and_records_nested_exception(self):
        recorder = Recorder()
        calls = []

        def operation(value):
            calls.append(value)
            return value + 1
        with recorder.span("outer"):
            self.assertEqual(recorder.wrap("inner", operation)(5), 6)
        self.assertEqual(calls, [5])
        self.assertEqual(recorder.events[0]["parent_id"], recorder.events[1]["id"])
        error = RuntimeError("fixture")
        with self.assertRaises(RuntimeError) as caught:
            with recorder.span("failed"):
                raise error
        self.assertIs(caught.exception, error)
        self.assertEqual(recorder.events[-1]["error_type"], "RuntimeError")
        self.assertEqual(recorder.stack, [])

    def test_warmup_errors_and_routes_are_not_pooled(self):
        def row(route, phase, status, duration, error=None):
            return {"route": route, "phase": phase, "status": status, "events": [
                {"boundary": "rpc", "elapsed_ns": duration, "error_type": error}]}
        result = summarize([
            row("direct", "warmup", "PASS", 999), row("direct", "sample", "PASS", 10),
            row("direct", "sample", "FAIL", 20), row("direct", "sample", "FAIL", 30, "Timeout"),
            row("approved", "sample", "PASS", 50),
        ])
        direct = result["direct/rpc"]
        self.assertEqual((direct["attempts"], direct["errors"]), (3, 2))
        self.assertEqual(direct["successful"]["count"], 1)
        self.assertEqual(direct["unsuccessful"]["p50_ns"], 25)
        self.assertEqual(result["approved/rpc"]["successful"]["p50_ns"], 50)

    def test_artifact_denominators_duplicates_and_failure_status(self):
        rows = [{"route": route, "phase": phase, "index": 0, "status": "PASS", "events": [
            {"boundary": "confirm_through_commit", "elapsed_ns": 1, "error_type": None}]}
                for route in ("direct", "approved") for phase in ("warmup", "sample")]
        result = {"schema": "eval03.v1", "kind": "smoke", "status": "PASS", "rows": rows,
                  "source_sha256": {"fixture": "test"}, "planned_per_route": {"sample": 1, "warmup": 1}}
        summary_module.validate(result)
        for malformed in ({**result, "status": "FAIL"}, {**result, "rows": rows[:-1]},
                          {**result, "rows": rows + [rows[0]]}, {**result, "source_sha256": {}}):
            with self.assertRaises(ValueError):
                summary_module.validate(malformed)

    def test_success_status_cannot_hide_unresolved_timer(self):
        for duration in (0, -1, True, 1.5, None):
            artifact = {"schema": "eval03.v1", "kind": "measurement", "status": "PASS",
                        "source_sha256": {"fixture": "test"}, "rows": [{"status": "PASS",
                        "events": [{"elapsed_ns": duration, "error_type": None}]}]}
            with self.assertRaisesRegex(ValueError, "timer sample"):
                summary_module.validate(artifact)

    def test_zero_control_is_retained_but_zero_api_and_invalid_control_are_rejected(self):
        names = ("timer_control", "evaluator_single_permit", "proof_v2_verify", "capability_v1_verify")
        rows = [{"route": "go_micro", "phase": phase, "index": 0, "status": "PASS", "events": [
            {"boundary": name, "elapsed_ns": 0 if name == "timer_control" else 100,
             "error_type": None}]} for phase in ("warmup", "sample") for name in names]
        artifact = {"schema": "eval03.v1", "kind": "measurement", "status": "PASS",
                    "source_sha256": {"fixture": "test"}, "rows": rows,
                    "planned_per_boundary": {"warmup": 1, "sample": 1}}
        summary_module.validate(artifact)
        control = summarize(rows)["go_micro/timer_control"]
        self.assertEqual((control["attempts"], control["errors"], control["successful"]["p50_ns"]), (1, 0, 0))
        for name in names[1:]:
            broken = deepcopy(artifact)
            next(r for r in broken["rows"] if r["events"][0]["boundary"] == name)["events"][0]["elapsed_ns"] = 0
            with self.assertRaisesRegex(ValueError, "timer sample"):
                summary_module.validate(broken)
        for duration in (-1, True, 1.5, None):
            broken = deepcopy(artifact)
            broken["rows"][0]["events"][0]["elapsed_ns"] = duration
            with self.assertRaisesRegex(ValueError, "timer sample"):
                summary_module.validate(broken)
        for change in ({"route": "direct"}, {"status": "FAIL"}):
            broken = deepcopy(artifact)
            broken["rows"][0].update(change)
            with self.assertRaises(ValueError):
                summary_module.validate(broken)


if __name__ == "__main__":
    unittest.main()
