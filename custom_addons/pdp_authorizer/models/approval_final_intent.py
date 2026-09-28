"""Locked current-intent preparation for a future approved PO final route."""

import base64

from odoo import models
from odoo.exceptions import AccessError

from ..cbi_protocol import (
    canonical_business_intent_bytes,
    canonical_business_intent_hash,
)
from .cbi_builder import build_purchase_order_intent


class PDPApprovalFinalIntent(models.Model):
    _inherit = "pdp.approval.request"

    def _prepare_final_intent(self):
        """Return the exact locked CBI, or invalidate a stale approved intent.

        The caller must keep this transaction and its locks through the final
        authority check and mutation. This method neither authorizes nor
        executes the business effect.
        """
        self.ensure_one()
        record = self.sudo()
        record._lock_approval_context()
        if record.state != "approved":
            raise AccessError("Final intent requires an approved capability.")

        attempt = record.authorization_attempt_id
        self.env.cr.execute(
            "SELECT id FROM pdp_authorization_attempt WHERE id = %s FOR UPDATE",
            [attempt.id],
        )
        if not self.env.cr.fetchone():
            raise AccessError("Approved command has no persisted authorization attempt.")
        attempt.invalidate_recordset()
        order = record.purchase_order_id
        grant = record.delegation_grant_id
        if (
            attempt.state != "approval_required"
            or attempt.tenant_id != record.tenant_id
            or attempt.delegation_grant_id != grant
            or attempt.delegation_nonce != record.command_id
            or attempt.business_model != "purchase.order"
            or attempt.business_res_id != order.id
            or record.company_id != order.company_id
            or order.pdp_delegation_nonce != record.command_id
        ):
            raise AccessError("Approved capability and protected command disagree.")

        try:
            stored = record._stored_intent()
            encoded = base64.urlsafe_b64encode(
                canonical_business_intent_bytes(stored)
            ).rstrip(b"=").decode("ascii")
            if (
                encoded != record.canonical_intent_b64
                or canonical_business_intent_hash(stored) != record.intent_hash
                or stored["state_witness"] != record.state_witness
                or stored["record_state"] != "to approve"
                or stored["tenant_id"] != record.tenant_id
                or stored["company_id"] != record.company_id.id
                or stored["resource_id"] != order.id
                or stored["delegation_grant_id"] != grant.id
                or stored["command_id"] != record.command_id
            ):
                raise AccessError("Stored approval intent has inconsistent binding.")
        except (KeyError, TypeError, ValueError) as exc:
            raise AccessError("Stored approval intent is invalid.") from exc

        try:
            current = build_purchase_order_intent(order, grant, record.command_id)
        except (AccessError, TypeError, ValueError):
            current = None
        if (
            current != stored
            or current is None
            or current["record_state"] != "to approve"
            or current["state_witness"] != record.state_witness
            or canonical_business_intent_hash(current) != record.intent_hash
        ):
            record._set_terminal("invalidated", "intent_changed")
            return False
        return current
