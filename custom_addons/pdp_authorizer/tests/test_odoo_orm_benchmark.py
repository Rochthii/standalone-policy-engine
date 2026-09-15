"""Real Odoo ORM/PostgreSQL versus mTLS gRPC PDP comparison."""

import json
import os
import platform
import statistics
import time
from datetime import timedelta

from odoo import Command, fields
from odoo.tests import TransactionCase, tagged

from ..models.pdp_client import SafePDPClient
from ..pdp_protocol import issue_jwt_from_environment


SAMPLES = 3


def _percentile(values, percentile):
    ordered = sorted(values)
    position = (len(ordered) - 1) * percentile
    lower = int(position)
    upper = min(lower + 1, len(ordered) - 1)
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


@tagged("post_install", "-at_install", "pdp_benchmark")
class TestOdooORMBenchmark(TransactionCase):
    """Measure one low-value PO authorization decision through two real paths."""

    @classmethod
    def setUpClass(cls):
        super().setUpClass()
        cls.iterations = int(os.environ.get("PDP_BENCHMARK_ITERATIONS", "250"))
        if cls.iterations < 10:
            raise AssertionError("PDP_BENCHMARK_ITERATIONS must be at least 10")

        cls.env.company.write(
            {
                "pdp_tenant_id": os.environ["PDP_TENANT_ID"],
                "po_double_validation": "one_step",
            }
        )
        purchase_user = cls.env.ref("purchase.group_purchase_user")
        cls.creator = cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "ORM Benchmark Creator",
                "login": "orm_benchmark_creator",
                "email": "orm-benchmark-creator@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "pdp_department": "Procurement",
            }
        )
        cls.approver = cls.env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "ORM Benchmark Approver",
                "login": "orm_benchmark_approver",
                "email": "orm-benchmark-approver@example.test",
                "company_id": cls.env.company.id,
                "company_ids": [Command.set([cls.env.company.id])],
                "pdp_department": "Procurement",
            }
        )
        cls.creator.write({"groups_id": [Command.link(purchase_user.id)]})
        cls.approver.write({"groups_id": [Command.link(purchase_user.id)]})
        cls.env["ir.rule"].sudo().create(
            {
                "name": "PDP benchmark low-value non-self approval",
                "model_id": cls.env.ref("purchase.model_purchase_order").id,
                "domain_force": "[('create_uid', '!=', user.id), ('amount_total', '<=', 2000)]",
                "groups": [Command.link(purchase_user.id)],
                "perm_read": True,
                "perm_write": False,
                "perm_create": False,
                "perm_unlink": False,
            }
        )
        cls.env["ir.rule"].clear_caches()
        cls.vendor = cls.env["res.partner"].create(
            {"name": "ORM Benchmark Vendor", "supplier_rank": 1}
        )
        cls.product = cls.env["product.product"].create(
            {"name": "ORM Benchmark Product", "purchase_ok": True}
        )
        cls.order = cls.env["purchase.order"].with_user(cls.creator).create(
            {
                "partner_id": cls.vendor.id,
                "company_id": cls.env.company.id,
                "pdp_department": "Procurement",
                "order_line": [
                    Command.create(
                        {
                            "name": cls.product.display_name,
                            "product_id": cls.product.id,
                            "product_qty": 1,
                            "product_uom": cls.product.uom_po_id.id,
                            "price_unit": 1000,
                            "date_planned": fields.Datetime.now(),
                        }
                    )
                ],
            }
        )
        cls.order.flush_recordset()

        cls.order.write(
            {
                "ai_agent_id": "agent:procurement_copilot",
                "delegated_by_id": cls.approver.id,
            }
        )
        cls.grant = cls.env["pdp.delegation.grant"].create(
            {
                "user_id": cls.approver.id,
                "agent_id": "agent:procurement_copilot",
                "max_amount": 2000,
                "valid_from": fields.Datetime.now() - timedelta(minutes=1),
                "valid_until": fields.Datetime.now() + timedelta(minutes=10),
            }
        )
        cls.grant.action_activate()
        values, proof, _ = cls.grant.build_protected_tuple(
            cls.order, "odoo-orm-benchmark-nonce"
        )
        request = cls.order._base_request(values["agent"])
        request["resource"] = values["resource"]
        request["context"].update(
            {
                "amount": values["amount"],
                "delegation_grant_id": values["grant_id"],
                "delegated_by": values["delegator"],
                "delegation_issued_at": str(values["issued_at"]),
                "delegation_valid_until": str(values["valid_until"]),
                "delegation_nonce": values["nonce"],
                "delegation_chain": values["delegation_chain"],
                "delegation_proof": proof,
                "resource.creator_id": values["creator_id"],
                "tool_context": values["tool_context"],
                "execution_mode": values["execution_mode"],
            }
        )
        cls.pdp_request = request
        cls.token = issue_jwt_from_environment(
            request["subject"], cls.env.company.pdp_tenant_id
        )

    def _native_orm_authorize(self):
        result = self.env["purchase.order"].with_user(self.approver).search(
            [("id", "=", self.order.id)], limit=1
        )
        self.assertEqual(result.ids, [self.order.id])

    def _pdp_authorize(self, client):
        decision, obligations, _ = client.check_access(
            self.env.company.pdp_tenant_id,
            self.pdp_request["subject"],
            self.pdp_request["action"],
            self.pdp_request["resource"],
            self.pdp_request["context"],
            self.token,
        )
        self.assertEqual(decision, "ALLOW")
        self.assertFalse(obligations)

    def _measure(self, operation):
        samples = []
        for _ in range(SAMPLES):
            elapsed_ns = []
            for _ in range(self.iterations):
                started = time.perf_counter_ns()
                operation()
                elapsed_ns.append(time.perf_counter_ns() - started)
            samples.append(elapsed_ns)
        flattened = [value for sample in samples for value in sample]
        return {
            "iterations_per_sample": self.iterations,
            "sample_count": SAMPLES,
            "raw_latency_ns": samples,
            "p50_ns": _percentile(flattened, 0.50),
            "p95_ns": _percentile(flattened, 0.95),
            "p99_ns": _percentile(flattened, 0.99),
            "mean_ns": statistics.mean(flattened),
            "throughput_ops_per_second": 1_000_000_000 / statistics.mean(flattened),
        }

    def test_real_orm_database_and_pdp_comparison(self):
        self._native_orm_authorize()
        client = SafePDPClient(timeout=1.0)
        try:
            self._pdp_authorize(client)
            native = self._measure(self._native_orm_authorize)
            pdp = self._measure(lambda: self._pdp_authorize(client))
        finally:
            client._channel.close()

        result = {
            "benchmark": "odoo_orm_record_rule_vs_mtls_grpc_pdp",
            "methodology": {
                "operation": "authorize one existing low-value purchase order created by a different user",
                "native_orm": "Odoo purchase.order search with a real ir.rule and PostgreSQL",
                "pdp": "Odoo generated Python client over mTLS gRPC with JWT and full-tuple delegation proof",
                "setup_and_one_warmup_per_path_excluded": True,
                "mutations_excluded": True,
            },
            "environment": {
                "python": platform.python_version(),
                "platform": platform.platform(),
                "odoo_database": self.env.cr.dbname,
                "git_commit": os.environ.get("PDP_GIT_COMMIT", "unknown"),
            },
            "native_orm": native,
            "pdp": pdp,
        }
        output_path = os.environ.get("PDP_BENCHMARK_OUTPUT", "").strip()
        if output_path:
            with open(output_path, "w", encoding="utf-8") as output:
                json.dump(result, output, indent=2, sort_keys=True)
        print("ODOO_ORM_BENCHMARK_RESULT=" + json.dumps(result, sort_keys=True), flush=True)
