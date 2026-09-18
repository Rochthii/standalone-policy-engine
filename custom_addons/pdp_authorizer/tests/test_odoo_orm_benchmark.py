"""Real Odoo purchase-confirmation transaction versus PDP PEP comparison."""

import json
import os
import platform
import statistics
import time
from datetime import timedelta

from odoo import Command, fields
from odoo.tests import TransactionCase, tagged

from ..models.purchase_order import PurchaseOrder as PDPPurchaseOrder


SAMPLES = 3


def _percentile(values, percentile):
    ordered = sorted(values)
    position = (len(ordered) - 1) * percentile
    lower = int(position)
    upper = min(lower + 1, len(ordered) - 1)
    return ordered[lower] + (ordered[upper] - ordered[lower]) * (position - lower)


@tagged("post_install", "-at_install", "pdp_benchmark")
class TestOdooORMBenchmark(TransactionCase):
    """Measure low-value purchase confirmation through Odoo and PDP PEP paths."""

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

    def _new_draft_order(self, delegated):
        values = {
            "partner_id": self.vendor.id,
            "company_id": self.env.company.id,
            "pdp_department": "Procurement",
            "order_line": [
                Command.create(
                    {
                        "name": self.product.display_name,
                        "product_id": self.product.id,
                        "product_qty": 1,
                        "product_uom": self.product.uom_po_id.id,
                        "price_unit": 1000,
                        "date_planned": fields.Datetime.now(),
                    }
                )
            ],
        }
        if delegated:
            values.update(
                {
                    "ai_agent_id": "agent:procurement_copilot",
                    "delegated_by_id": self.approver.id,
                    "delegation_grant_id": self.grant.id,
                }
            )
        return self.env["purchase.order"].with_user(self.creator).create(values)

    def _native_confirm_transaction(self):
        order = self._new_draft_order(delegated=False).with_user(self.approver)
        super(PDPPurchaseOrder, order).button_confirm()
        order.flush_recordset()
        self.assertIn(order.state, ("purchase", "done"))

    def _pdp_confirm_transaction(self):
        order = self._new_draft_order(delegated=True)
        order.button_confirm()
        order.flush_recordset()
        self.assertEqual(order.pdp_status, "allow")
        self.assertIn(order.state, ("purchase", "done"))

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

    def test_real_erp_transaction_and_pdp_comparison(self):
        self._native_confirm_transaction()
        self._pdp_confirm_transaction()
        native = self._measure(self._native_confirm_transaction)
        pdp = self._measure(self._pdp_confirm_transaction)

        result = {
            "benchmark": "odoo_purchase_confirmation_vs_mtls_grpc_pdp",
            "methodology": {
                "operation": "create and confirm one low-value purchase order",
                "native_orm": "Odoo purchase.order button_confirm with PostgreSQL",
                "pdp": "Odoo PDP PEP button_confirm with mTLS gRPC, JWT and full-tuple delegation proof",
                "setup_and_one_warmup_per_path_excluded": True,
                "business_mutation_included": True,
                "database_commit_excluded": True,
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
