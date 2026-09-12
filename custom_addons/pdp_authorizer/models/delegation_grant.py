import calendar
import math

from odoo import _, api, fields, models
from odoo.exceptions import UserError

from ..pdp_protocol import (
    MAX_DELEGATION_TTL_SECONDS,
    delegation_fingerprint,
    issue_jwt_from_environment,
    load_delegation_keyring,
    sign_delegation_tuple,
)
from .pdp_client import get_pdp_client


def _unix_seconds(value):
    value = fields.Datetime.to_datetime(value)
    return calendar.timegm(value.utctimetuple()) if value else 0


class PDPDelegationGrant(models.Model):
    _name = "pdp.delegation.grant"
    _description = "AI Agent Delegation Grant"
    _order = "id desc"

    name = fields.Char(compute="_compute_name", store=True)
    user_id = fields.Many2one(
        "res.users", required=True, default=lambda self: self.env.user, index=True
    )
    agent_id = fields.Char(required=True, default="agent:procurement_copilot", index=True)
    max_amount = fields.Float(required=True, default=2000.0)
    valid_from = fields.Datetime(required=True, default=fields.Datetime.now)
    valid_until = fields.Datetime(required=True)
    state = fields.Selection(
        [
            ("draft", "Draft"),
            ("active", "Active"),
            ("revoked", "Revoked"),
            ("expired", "Expired"),
        ],
        required=True,
        default="draft",
        index=True,
        copy=False,
    )
    notes = fields.Text()

    @api.depends("user_id", "agent_id")
    def _compute_name(self):
        for grant in self:
            grant.name = "GRANT-%s-%s" % (grant.id or "NEW", grant.agent_id or "agent")

    def _tenant_id(self):
        self.ensure_one()
        tenant_id = (self.user_id.company_id.pdp_tenant_id or "").strip()
        if not tenant_id:
            raise UserError(_("The delegator company has no PDP Tenant ID configured."))
        return tenant_id

    def action_activate(self):
        for grant in self:
            issued_at = _unix_seconds(grant.valid_from)
            valid_until = _unix_seconds(grant.valid_until)
            if valid_until <= issued_at:
                raise UserError(_("Delegation expiry must be after its start time."))
            if valid_until - issued_at > MAX_DELEGATION_TTL_SECONDS:
                raise UserError(_("Delegation validity cannot exceed 24 hours."))
            load_delegation_keyring()
            grant.write({"state": "active"})

    def action_revoke(self):
        for grant in self:
            tenant_id = grant._tenant_id()
            subject = "user:%s" % grant.env.user.login
            token = issue_jwt_from_environment(
                subject, tenant_id, permissions=("delegation:revoke",)
            )
            get_pdp_client().revoke_delegation(
                tenant_id, str(grant.id), subject, token, "Revoked from Odoo"
            )
            grant.write({"state": "revoked"})

    def build_protected_tuple(self, order, nonce):
        self.ensure_one()
        if self.state != "active":
            raise UserError(_("The selected delegation grant is not active."))
        if not order.ai_agent_id or order.ai_agent_id != self.agent_id:
            raise UserError(_("The purchase order agent does not match the delegation grant."))
        tenant_id = self._tenant_id()
        delegator = "user:%s" % self.user_id.login
        creator = "user:%s" % order.create_uid.login
        values = {
            "tenant_id": tenant_id,
            "grant_id": str(self.id),
            "delegator": delegator,
            "agent": self.agent_id,
            "action": "action:APPROVE_PURCHASE_ORDER",
            "resource": "purchase_order:%s" % order.id,
            "amount": str(int(math.ceil(order.amount_total))),
            "delegation_chain": "%s,%s" % (delegator, self.agent_id),
            "creator_id": creator,
            "tool_context": "tool:auto_confirm_po",
            "execution_mode": "autonomous_run",
            "nonce": nonce,
            "issued_at": _unix_seconds(self.valid_from),
            "valid_until": _unix_seconds(self.valid_until),
        }
        key_id, keys = load_delegation_keyring()
        proof = sign_delegation_tuple(values, key_id, keys[key_id])
        return values, proof, delegation_fingerprint(values, key_id)
