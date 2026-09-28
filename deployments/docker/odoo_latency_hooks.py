"""Transparent, process-local timing wrappers for the isolated EVAL-03 runner."""
from contextlib import contextmanager, ExitStack
from unittest.mock import patch


@contextmanager
def instrument(recorder):
    from odoo.addons.pdp_authorizer.models import (
        approval_final_intent, approval_invalidation, cbi_builder, delegation_grant,
        purchase_order, pdp_client,
    )
    from odoo.addons.purchase.models.purchase_order import PurchaseOrder

    # Patch imported aliases, not just the defining module. Every wrapper invokes
    # the original unchanged; no sentinel injection, mocked ALLOW or bypass.
    targets = [(module, "build_purchase_order_intent", "locked_cbi") for module in (
        cbi_builder, delegation_grant, purchase_order, approval_final_intent, approval_invalidation)]
    targets += [
        (delegation_grant, "sign_delegation_proof_v2", "python_proof_sign"),
        (pdp_client.SafePDPClient, "check_access", "rpc_check_access"),
        (pdp_client.SafePDPClient, "verify_approval_capability", "rpc_verify_capability"),
        (PurchaseOrder, "button_approve", "native_button_approve"),
    ]
    with ExitStack() as stack:
        for owner, name, boundary in targets:
            stack.enter_context(patch.object(owner, name, recorder.wrap(boundary, getattr(owner, name))))
        yield
