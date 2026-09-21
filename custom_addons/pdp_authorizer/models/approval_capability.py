"""ApprovalCapability v1 issuance for one locked pending Odoo intent."""

import base64
import calendar
import hashlib
from datetime import datetime

from odoo import fields, models
from odoo.exceptions import AccessError

from ..pdp_protocol import issue_jwt_from_environment
from .pdp_client import get_pdp_client


CAPABILITY_VERSION = "ac.v1"
CAPABILITY_PURPOSE = "odoo.purchase_order.confirm"
CAPABILITY_PERMISSION = "approval:purchase_order.confirm"
CAPABILITY_ALGORITHM = "HS256"


class PDPApprovalCapability(models.Model):
    _inherit = "pdp.approval.request"

    approver_user_id = fields.Many2one(
        "res.users", index=True, readonly=True, ondelete="restrict"
    )
    approver_subject = fields.Char(readonly=True)
    required_permission = fields.Char(readonly=True)
    issuance_policy_revision = fields.Integer(readonly=True)
    issued_at = fields.Datetime(readonly=True)
    expires_at = fields.Datetime(readonly=True)
    one_time_id = fields.Char(index=True, readonly=True)
    capability_version = fields.Char(readonly=True)
    capability_purpose = fields.Char(readonly=True)
    capability_algorithm = fields.Char(readonly=True)
    capability_key_id = fields.Char(readonly=True)
    capability_payload_b64 = fields.Text(readonly=True)
    capability_signature_b64 = fields.Char(readonly=True)
    capability_fingerprint = fields.Char(index=True, readonly=True)

    _sql_constraints = [
        (
            "approval_one_time_id_uniq",
            "unique(tenant_id, one_time_id)",
            "This approval capability one-time ID already exists.",
        ),
    ]

    def action_issue_capability(self):
        """Issue one capability; never apply the protected purchase-order effect."""
        self.ensure_one()
        approver = self.env.user
        record = self.sudo()
        self.env.cr.execute(
            "SELECT id FROM purchase_order WHERE id = %s FOR UPDATE",
            [record.purchase_order_id.id],
        )
        self.env.cr.execute(
            "SELECT id FROM pdp_approval_request WHERE id = %s FOR UPDATE",
            [record.id],
        )
        record.invalidate_recordset()
        if record.state == "approved":
            return record._verify_idempotent_retry(approver)

        authority = self.with_user(approver).check_current_approver()
        intent = record._validated_pending_intent()
        grant = record.delegation_grant_id
        valid_until = _unix_seconds(grant.valid_until)
        if grant.state != "active" or valid_until <= _unix_seconds(fields.Datetime.now()):
            raise AccessError("Delegation is not active at approval issuance.")

        token = record._approver_token(approver, authority["approver_subject"])
        capability = get_pdp_client().issue_approval_capability(
            record._issuance_values(intent, authority, valid_until), token
        )
        record._validate_issued_binding(capability, intent, authority)
        get_pdp_client().verify_approval_capability(record.tenant_id, capability, token)
        record.write(record._capability_write_values(capability, approver))
        return True

    def _verify_idempotent_retry(self, approver):
        self.ensure_one()
        if self.approver_user_id != approver or not self.capability_payload_b64:
            raise AccessError("Approval was already issued to a different or invalid identity.")
        capability = self._stored_capability()
        token = self._approver_token(approver, self.approver_subject)
        get_pdp_client().verify_approval_capability(self.tenant_id, capability, token)
        return True

    def _approver_token(self, approver, subject):
        return issue_jwt_from_environment(
            subject,
            self.tenant_id,
            attributes={
                "department": approver.pdp_department,
                "company_id": str(approver.company_id.id),
            },
        )

    def _issuance_values(self, intent, authority, valid_until):
        return {
            "tenant_id": self.tenant_id,
            "company_id": self.company_id.id,
            "approval_id": self.approval_id,
            "intent_hash": self.intent_hash,
            "state_witness": self.state_witness,
            "command_id": self.command_id,
            "delegation_grant_id": self.delegation_grant_id.id,
            "delegator_subject": intent["delegator_subject"],
            "agent_subject": intent["agent_subject"],
            "creator_subject": intent["creator_subject"],
            "approver_user_id": authority["approver_user_id"],
            "resource": "purchase_order:%s" % self.purchase_order_id.id,
            "delegation_valid_until": valid_until,
            "context": {
                "resource.department": self.purchase_order_id.pdp_department,
            },
        }

    def _validate_issued_binding(self, capability, intent, authority):
        expected = {
            "capability_version": CAPABILITY_VERSION,
            "purpose": CAPABILITY_PURPOSE,
            "approval_id": self.approval_id,
            "tenant_id": self.tenant_id,
            "company_id": self.company_id.id,
            "intent_hash": self.intent_hash,
            "state_witness": self.state_witness,
            "command_id": self.command_id,
            "delegation_grant_id": self.delegation_grant_id.id,
            "delegator_subject": intent["delegator_subject"],
            "agent_subject": intent["agent_subject"],
            "creator_subject": intent["creator_subject"],
            "approver_user_id": authority["approver_user_id"],
            "approver_subject": authority["approver_subject"],
            "required_permission": CAPABILITY_PERMISSION,
            "algorithm": CAPABILITY_ALGORITHM,
        }
        if any(capability.get(key) != value for key, value in expected.items()):
            raise AccessError("Issued capability does not match the pending approval.")
        if (
            capability["issuance_policy_revision"] < 0
            or capability["issued_at"] <= 0
            or capability["expires_at"] <= capability["issued_at"]
            or not capability["one_time_id"]
            or not capability["key_id"]
            or not capability["canonical_payload"]
            or not capability["signature"]
        ):
            raise AccessError("Issued capability metadata is invalid.")

    def _capability_write_values(self, capability, approver):
        payload = capability["canonical_payload"]
        signature = capability["signature"]
        return {
            "state": "approved",
            "approver_user_id": approver.id,
            "approver_subject": capability["approver_subject"],
            "required_permission": capability["required_permission"],
            "issuance_policy_revision": capability["issuance_policy_revision"],
            "issued_at": _odoo_datetime(capability["issued_at"]),
            "expires_at": _odoo_datetime(capability["expires_at"]),
            "one_time_id": capability["one_time_id"],
            "capability_version": capability["capability_version"],
            "capability_purpose": capability["purpose"],
            "capability_algorithm": capability["algorithm"],
            "capability_key_id": capability["key_id"],
            "capability_payload_b64": _base64url(payload),
            "capability_signature_b64": _base64url(signature),
            "capability_fingerprint": hashlib.sha256(payload + signature).hexdigest(),
        }

    def _stored_capability(self):
        intent = self._stored_intent()
        return {
            "capability_version": self.capability_version,
            "purpose": self.capability_purpose,
            "approval_id": self.approval_id,
            "tenant_id": self.tenant_id,
            "company_id": self.company_id.id,
            "intent_hash": self.intent_hash,
            "state_witness": self.state_witness,
            "command_id": self.command_id,
            "delegation_grant_id": self.delegation_grant_id.id,
            "delegator_subject": intent["delegator_subject"],
            "agent_subject": intent["agent_subject"],
            "creator_subject": intent["creator_subject"],
            "approver_user_id": self.approver_user_id.id,
            "approver_subject": self.approver_subject,
            "required_permission": self.required_permission,
            "issuance_policy_revision": self.issuance_policy_revision,
            "issued_at": _unix_seconds(self.issued_at),
            "expires_at": _unix_seconds(self.expires_at),
            "one_time_id": self.one_time_id,
            "algorithm": self.capability_algorithm,
            "key_id": self.capability_key_id,
            "canonical_payload": _decode_base64url(self.capability_payload_b64),
            "signature": _decode_base64url(self.capability_signature_b64),
        }

    def _stored_intent(self):
        import json

        return json.loads(self.intent_json)


def _unix_seconds(value):
    parsed = fields.Datetime.to_datetime(value)
    return calendar.timegm(parsed.utctimetuple()) if parsed else 0


def _odoo_datetime(value):
    return fields.Datetime.to_string(datetime.utcfromtimestamp(value))


def _base64url(value):
    return base64.urlsafe_b64encode(value).rstrip(b"=").decode("ascii")


def _decode_base64url(value):
    return base64.urlsafe_b64decode(value + "=" * (-len(value) % 4))
