"""Test-runner-only timing and raw-sample summaries; no Odoo dependency."""
from contextlib import contextmanager
from functools import wraps
import time


def distribution(values):
    """Linear interpolation at (n-1)*p; empty sets have no latency estimate."""
    if not values:
        return {"count": 0, "p50_ns": None, "p95_ns": None, "p99_ns": None, "max_ns": None}
    if any(type(value) is not int or value < 0 for value in values):
        raise ValueError("Durations must be nonnegative integer nanoseconds")
    ordered = sorted(values)

    def percentile(p):
        position = (len(ordered) - 1) * p
        lower = int(position)
        upper = min(lower + 1, len(ordered) - 1)
        return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)

    return {"count": len(values), "p50_ns": percentile(.50), "p95_ns": percentile(.95),
            "p99_ns": percentile(.99), "max_ns": ordered[-1]}


class Recorder:
    def __init__(self):
        self.events = []
        self.stack = []
        self.sequence = 0

    @contextmanager
    def span(self, boundary):
        self.sequence += 1
        event = {"id": self.sequence, "parent_id": self.stack[-1] if self.stack else None,
                 "boundary": boundary, "error_type": None}
        self.stack.append(event["id"])
        started = time.perf_counter_ns()
        try:
            yield
        except BaseException as error:
            event["error_type"] = type(error).__name__
            raise
        finally:
            event["elapsed_ns"] = time.perf_counter_ns() - started
            self.stack.pop()
            self.events.append(event)

    def wrap(self, boundary, operation):
        @wraps(operation)
        def measured(*args, **kwargs):
            with self.span(boundary):
                return operation(*args, **kwargs)
        return measured


def summarize(rows):
    """Keep failed transactions out of success percentiles, even if an inner call passed."""
    groups = {}
    for row in rows:
        if row["phase"] != "sample":
            continue
        for event in row["events"]:
            key = row["route"] + "/" + event["boundary"]
            group = groups.setdefault(key, {"attempts": 0, "errors": 0,
                                           "successful_ns": [], "unsuccessful_ns": []})
            failed = row["status"] != "PASS" or event["error_type"] is not None
            group["attempts"] += 1
            group["errors"] += int(failed)
            group["unsuccessful_ns" if failed else "successful_ns"].append(event["elapsed_ns"])
    return {key: {"attempts": group["attempts"], "errors": group["errors"],
                  "successful": distribution(group["successful_ns"]),
                  "unsuccessful": distribution(group["unsuccessful_ns"])}
            for key, group in sorted(groups.items())}
