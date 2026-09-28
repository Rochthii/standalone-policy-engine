"""Recompute summaries from EVAL-03 raw files, preserving run and route separation."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path

from eval_latency_metrics import summarize


def validate(result):
    if result.get("schema") != "eval03.v1" or result.get("status") != "PASS":
        raise ValueError("Only completed eval03.v1 runs may be summarized as accepted measurements")
    if result.get("kind") not in ("smoke", "measurement") or not result.get("source_sha256"):
        raise ValueError("Missing run kind or source fingerprint")
    rows = result["rows"]
    for row in rows:
        for event in row["events"]:
            elapsed = event.get("elapsed_ns")
            control = ("planned_per_boundary" in result and "planned_per_route" not in result
                       and row.get("route") == "go_micro" and event.get("boundary") == "timer_control")
            if type(elapsed) is not int or elapsed < 0 or (elapsed == 0 and not control):
                raise ValueError("Unresolved/invalid timer sample; zero allowed only for Go timer_control")
    if any(row["status"] != "PASS" or any(event["error_type"] for event in row["events"]) for row in rows):
        raise ValueError("PASS artifact contains failed samples")
    if "planned_per_route" in result:
        actual = Counter((row["route"], row["phase"]) for row in rows)
        expected = Counter({(route, phase): count for route in ("direct", "approved")
                            for phase, count in result["planned_per_route"].items()})
    else:
        actual = Counter((event["boundary"], row["phase"]) for row in rows for event in row["events"])
        expected = Counter({(boundary, phase): count for boundary in (
            "timer_control", "evaluator_single_permit", "proof_v2_verify", "capability_v1_verify")
                            for phase, count in result["planned_per_boundary"].items()})
    if actual != expected:
        raise ValueError("Raw denominators do not match the planned sample/warm-up counts")
    if len({(row["route"], row["phase"], row["index"], tuple(e["boundary"] for e in row["events"]))
            for row in rows}) != len(rows):
        raise ValueError("Duplicate sample identity")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("inputs", nargs="+", type=Path)
    parser.add_argument("--output", required=True, type=Path, help="New path; refuses overwrite")
    args = parser.parse_args()
    output = []
    for path in args.inputs:
        data = path.read_bytes()
        result = json.loads(data)
        validate(result)
        summary = summarize(result["rows"])
        if "summary" in result and summary != result["summary"]:
            raise ValueError("Stored summary differs from raw samples: " + str(path))
        output.append({"source": str(path), "sha256": hashlib.sha256(data).hexdigest(),
                       "kind": result["kind"], "summary": summary})
    with args.output.open("x", encoding="utf-8") as stream:
        json.dump({"schema": "eval03-summary.v1", "runs": output}, stream, indent=2)
        stream.write("\n")
    print("Validated %d separate runs; summary: %s" % (len(output), args.output))


if __name__ == "__main__":
    main()
