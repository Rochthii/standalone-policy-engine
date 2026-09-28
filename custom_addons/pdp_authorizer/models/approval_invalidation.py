"""Locked invalidation lifecycle for an issued ApprovalCapability v1."""

from odoo import fields, models
from odoo.exceptions import AccessError

from ..cbi_protocol import canonical_business_intent_hash
from ..pdp_protocol import issue_jwt_from_environment
from .approval_request import APPROVAL_ACTION
from .cbi_builder import build_purchase_order_intent
from .pdp_client import PDPUnavailableError, get_pdp_client


TERMINAL_STATES = frozenset({"rejected", "invalidated", "expired", "consumed"})


class PDPApprovalInvalidation(models.Model):
    _inherit = "pdp.approval.request"

    terminal_reason = fields.Char(readonly=True, copy=False)
    terminal_at = fields.Datetime(readonly=True, copy=False)

    def _revalidate_approved_capability(self, policy_revisions=None):
        """Fail closed and persist only deterministic invalidation outcomes."""
        self.ensure_one()
        record = self.sudo()
        record._lock_approval_context()

        if record.state in TERMINAL_STATES:
            raise AccessError("Approval is already in an immutable terminal state.")
        if record.state != "approved":
            raise AccessError("Only an approved capability can be revalidated.")

        terminal = record._deterministic_binding_failure()
        if terminal:
            record._set_terminal(*terminal)
            return False

        try:
            valid = record._verify_current_approval_authority(policy_revisions)
        except PDPUnavailableError:
            raise
        except AccessError:
            record._set_terminal("invalidated", "capability_verification_failed")
            return False
        if not valid:
            record._set_terminal("invalidated", "current_approver_authority_denied")
            return False
        return True

    def _lock_approval_context(self):
        self.ensure_one()
        self.env.cr.execute(
            "SELECT id FROM purchase_order WHERE id = %s FOR UPDATE",
            [self.purchase_order_id.id],
        )
        if not self.env.cr.fetchone():
            raise AccessError("Approval purchase order no longer exists.")
        self.env.cr.execute(
            "SELECT id FROM pdp_approval_request WHERE id = %s FOR UPDATE",
            [self.id],
        )
        if not self.env.cr.fetchone():
            raise AccessError("Approval request no longer exists.")
        self.invalidate_recordset()
        self.purchase_order_id.invalidate_recordset()
        self.delegation_grant_id.invalidate_recordset()

    def _deterministic_binding_failure(self):
        self.ensure_one()
        now = fields.Datetime.now()
        if not self.expires_at or self.expires_at <= now:
            return "expired", "capability_expired"

        grant = self.delegation_grant_id
        if grant.state == "revoked":
            return "invalidated", "delegation_revoked"
        if grant.state != "active":
            return "invalidated", "delegation_inactive"
        if not grant.valid_until or grant.valid_until <= now:
            return "invalidated", "delegation_expired"

        try:
            current = build_purchase_order_intent(
                self.purchase_order_id, grant, self.command_id
            )
            stored = self._stored_intent()
        except (AccessError, TypeError, ValueError):
            return "invalidated", "intent_changed"
        if (
            current != stored
            or canonical_business_intent_hash(current) != self.intent_hash
            or current["state_witness"] != self.state_witness
        ):
            return "invalidated", "intent_changed"
        return None

    def _verify_current_approval_authority(self, policy_revisions=None):
        self.ensure_one()
        approver = self.approver_user_id
        intent = self._stored_intent()
        if not self._has_current_local_authority(approver, intent):
            return False

        token = issue_jwt_from_environment(
            self.approver_subject,
            self.tenant_id,
            attributes={
                "department": approver.pdp_department,
                "company_id": str(approver.company_id.id),
            },
        )
        client = get_pdp_client()
        client.verify_approval_capability(
            self.tenant_id, self._stored_capability(), token
        )
        decision, obligations, advice = client.check_access(
            self.tenant_id,
            self.approver_subject,
            APPROVAL_ACTION,
            "purchase_order:%s" % self.purchase_order_id.id,
            {
                "approval_id": self.approval_id,
                "approval_state": "approved",
                "intent_hash": self.intent_hash,
                "state_witness": self.state_witness,
                "command_id": self.command_id,
                "grant_id": str(self.delegation_grant_id.id),
                "resource.creator_id": intent["creator_subject"],
                "resource.department": self.purchase_order_id.pdp_department,
            },
            token,
        )
        if policy_revisions is not None:
            policy_revisions.append(advice.get("erp.policy_revision"))
        return decision == "ALLOW" and not obligations

    def _has_current_local_authority(self, approver, intent):
        self.ensure_one()
        if (
            not approver
            or not approver.active
            or approver.share
            or not approver.login
            or self.approver_subject != "user:%s" % approver.login
            or approver.company_id != self.company_id
            or self.company_id not in approver.company_ids
            or (approver.company_id.pdp_tenant_id or "").strip() != self.tenant_id
            or not approver.with_user(approver).has_group(
                "purchase.group_purchase_manager"
            )
        ):
            return False
        return self.approver_subject not in {
            intent["agent_subject"],
            intent["creator_subject"],
            intent["delegator_subject"],
        } and approver.id not in {
            self.purchase_order_id.create_uid.id,
            self.delegation_grant_id.user_id.id,
        }

    def _set_terminal(self, state, reason):
        self.ensure_one()
        if state not in {"invalidated", "expired"}:
            raise AccessError("Unsupported approval terminal transition.")
        self.write(
            {
                "state": state,
                "terminal_reason": reason,
                "terminal_at": fields.Datetime.now(),
            }
        )
