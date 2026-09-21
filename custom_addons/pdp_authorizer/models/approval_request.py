"""Persistent pending approval intent for the bounded purchase-order workflow."""

import base64
import json
import secrets

from odoo import api, fields, models
from odoo.exceptions import AccessError

from ..cbi_protocol import (
    canonical_business_intent_bytes,
    canonical_business_intent_hash,
)
from ..pdp_protocol import issue_jwt_from_environment
from .pdp_client import get_pdp_client


APPROVAL_ACTION = "action:APPROVE_PURCHASE_ORDER"


class PDPApprovalRequest(models.Model):
    _name = "pdp.approval.request"
    _description = "PDP exact-action pending approval request"
    _order = "id desc"

    approval_id = fields.Char(required=True, index=True, readonly=True)
    tenant_id = fields.Char(required=True, index=True, readonly=True)
    company_id = fields.Many2one(
        "res.company", required=True, index=True, readonly=True, ondelete="restrict"
    )
    purchase_order_id = fields.Many2one(
        "purchase.order", required=True, index=True, readonly=True, ondelete="restrict"
    )
    authorization_attempt_id = fields.Many2one(
        "pdp.authorization.attempt",
        required=True,
        index=True,
        readonly=True,
        ondelete="restrict",
    )
    delegation_grant_id = fields.Many2one(
        "pdp.delegation.grant",
        required=True,
        index=True,
        readonly=True,
        ondelete="restrict",
    )
    command_id = fields.Char(required=True, index=True, readonly=True)
    intent_json = fields.Text(required=True, readonly=True)
    canonical_intent_b64 = fields.Text(required=True, readonly=True)
    intent_hash = fields.Char(required=True, index=True, readonly=True)
    state_witness = fields.Char(required=True, readonly=True)
    state = fields.Selection(
        [("pending", "Pending"), ("approved", "Approved")],
        required=True,
        default="pending",
        index=True,
        readonly=True,
    )
    requested_approver_user_id = fields.Many2one(
        "res.users", index=True, readonly=True, ondelete="restrict"
    )
    activity_id = fields.Many2one("mail.activity", readonly=True, ondelete="set null")

    _sql_constraints = [
        (
            "approval_id_uniq",
            "unique(approval_id)",
            "This approval ID already exists.",
        ),
        (
            "authorization_attempt_uniq",
            "unique(authorization_attempt_id)",
            "This authorization attempt already has an approval request.",
        ),
    ]

    @api.model
    def create_pending(self, order, attempt, intent):
        """Persist one validated post-transition CBI for one protected attempt."""
        order.ensure_one()
        attempt.ensure_one()
        canonical_bytes = canonical_business_intent_bytes(intent)
        intent_hash = canonical_business_intent_hash(intent)
        if (
            order.state != "to approve"
            or intent["record_state"] != "to approve"
            or intent["tenant_id"] != attempt.tenant_id
            or intent["company_id"] != order.company_id.id
            or intent["resource_id"] != order.id
            or intent["command_id"] != attempt.delegation_nonce
            or intent["delegation_grant_id"] != attempt.delegation_grant_id.id
        ):
            raise AccessError("Pending approval intent does not match its protected attempt.")
        return self.create(
            {
                "approval_id": secrets.token_urlsafe(16),
                "tenant_id": intent["tenant_id"],
                "company_id": intent["company_id"],
                "purchase_order_id": order.id,
                "authorization_attempt_id": attempt.id,
                "delegation_grant_id": intent["delegation_grant_id"],
                "command_id": intent["command_id"],
                "intent_json": json.dumps(
                    intent, ensure_ascii=False, sort_keys=True, separators=(",", ":")
                ),
                "canonical_intent_b64": base64.urlsafe_b64encode(canonical_bytes)
                .rstrip(b"=")
                .decode("ascii"),
                "intent_hash": intent_hash,
                "state_witness": intent["state_witness"],
            }
        )

    def select_activity_approver(self):
        """Select a same-company manager without granting approval authority."""
        self.ensure_one()
        intent = self._validated_pending_intent()
        manager_group = self.env.ref("purchase.group_purchase_manager")
        candidates = self.env["res.users"].sudo().search(
            [
                ("active", "=", True),
                ("share", "=", False),
                ("company_id", "=", self.company_id.id),
                ("groups_id", "in", [manager_group.id]),
            ],
            order="id",
        )
        disallowed = {
            intent["agent_subject"],
            intent["creator_subject"],
            intent["delegator_subject"],
        }
        for candidate in candidates:
            subject = "user:%s" % (candidate.login or "")
            tenant_id = (candidate.company_id.pdp_tenant_id or "").strip()
            is_system_admin = candidate.has_group("base.group_system")
            if (
                not is_system_admin
                and subject not in disallowed
                and tenant_id == self.tenant_id
            ):
                return candidate
        raise AccessError("No independent same-tenant purchase approver is available.")

    def attach_activity(self, activity, approver):
        self.ensure_one()
        activity.ensure_one()
        approver.ensure_one()
        if activity.user_id != approver:
            raise AccessError("Approval Activity recipient does not match the selected user.")
        if self.requested_approver_user_id and self.requested_approver_user_id != approver:
            raise AccessError("Pending approval already targets a different user.")
        if self.activity_id and self.activity_id != activity:
            raise AccessError("Pending approval already links a different Activity.")
        if not self.activity_id or not self.requested_approver_user_id:
            self.write(
                {
                    "activity_id": activity.id,
                    "requested_approver_user_id": approver.id,
                }
            )

    def check_current_approver(self):
        """Authorize the authenticated human without changing approval state."""
        self.ensure_one()
        approver = self.env.user
        record = self.sudo()
        intent = record._validated_pending_intent()
        subject = "user:%s" % (approver.login or "")
        if not approver.active or approver.share or not approver.login:
            raise AccessError("Approval requires an active internal human user.")
        if (
            approver.company_id != record.company_id
            or record.company_id not in approver.company_ids
            or (approver.company_id.pdp_tenant_id or "").strip() != record.tenant_id
        ):
            raise AccessError("Approver tenant or company does not match the pending intent.")
        if not approver.has_group("purchase.group_purchase_manager"):
            raise AccessError("Approver lacks the required purchase-manager role.")
        if subject in {
            intent["agent_subject"],
            intent["creator_subject"],
            intent["delegator_subject"],
        } or approver.id in {
            record.purchase_order_id.create_uid.id,
            record.delegation_grant_id.user_id.id,
        }:
            raise AccessError("Approver violates Separation of Duties.")

        token = issue_jwt_from_environment(
            subject,
            record.tenant_id,
            attributes={
                "department": approver.pdp_department,
                "company_id": str(approver.company_id.id),
            },
        )
        decision, obligations, _advice = get_pdp_client().check_access(
            record.tenant_id,
            subject,
            APPROVAL_ACTION,
            "purchase_order:%s" % record.purchase_order_id.id,
            {
                "approval_id": record.approval_id,
                "approval_state": record.state,
                "intent_hash": record.intent_hash,
                "state_witness": record.state_witness,
                "command_id": record.command_id,
                "grant_id": str(record.delegation_grant_id.id),
                "resource.creator_id": intent["creator_subject"],
                "resource.department": record.purchase_order_id.pdp_department,
            },
            token,
        )
        if decision != "ALLOW" or obligations:
            raise AccessError("PDP denied approval authority for this pending intent.")
        return {"approver_user_id": approver.id, "approver_subject": subject}

    def _validated_pending_intent(self):
        self.ensure_one()
        try:
            intent = json.loads(self.intent_json)
            canonical_bytes = canonical_business_intent_bytes(intent)
        except (TypeError, ValueError, json.JSONDecodeError) as exc:
            raise AccessError("Stored pending approval intent is invalid.") from exc
        encoded = base64.urlsafe_b64encode(canonical_bytes).rstrip(b"=").decode("ascii")
        if (
            self.state != "pending"
            or self.purchase_order_id.state != "to approve"
            or encoded != self.canonical_intent_b64
            or canonical_business_intent_hash(intent) != self.intent_hash
            or intent["state_witness"] != self.state_witness
            or intent["tenant_id"] != self.tenant_id
            or intent["company_id"] != self.company_id.id
            or intent["resource_id"] != self.purchase_order_id.id
            or intent["command_id"] != self.command_id
            or intent["delegation_grant_id"] != self.delegation_grant_id.id
        ):
            raise AccessError("Stored pending approval does not match its canonical intent.")
        return intent
