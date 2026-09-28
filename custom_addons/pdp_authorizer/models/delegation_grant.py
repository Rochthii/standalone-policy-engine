import calendar
import math
from decimal import Decimal

from odoo import _, api, fields, models
from odoo.exceptions import AccessError, UserError

from ..pdp_protocol import (
    MAX_DELEGATION_TTL_SECONDS,
    delegation_fingerprint,
    issue_jwt_from_environment,
    load_delegation_keyring,
    sign_delegation_tuple,
)
from ..cbi_protocol import amount_to_minor_units, canonical_business_intent_hash
from ..delegation_proof_v2 import sign_delegation_proof_v2
from .cbi_builder import build_purchase_order_intent
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
    currency_id = fields.Many2one(
        "res.currency",
        required=True,
        default=lambda self: self.env.company.currency_id,
    )
    max_amount = fields.Monetary(
        required=True, default=2000.0, currency_field="currency_id"
    )
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

    def _auto_init(self):
        cr = self.env.cr
        cr.execute("SELECT to_regclass('public.pdp_delegation_grant')")
        table_exists = cr.fetchone()[0] is not None
        currency_column_missing = False
        if table_exists:
            cr.execute(
                """
                SELECT NOT EXISTS (
                    SELECT 1 FROM information_schema.columns
                     WHERE table_schema = current_schema()
                       AND table_name = 'pdp_delegation_grant'
                       AND column_name = 'currency_id'
                )
                """
            )
            currency_column_missing = cr.fetchone()[0]

        result = super()._auto_init()
        if currency_column_missing:
            # Legacy max_amount had no denomination. On upgrade, interpret it
            # in the delegator's primary company currency, not the upgrader's
            # current company currency. Never overwrite explicitly configured
            # currencies on later module updates.
            cr.execute(
                """
                UPDATE pdp_delegation_grant AS legacy_grant
                   SET currency_id = company.currency_id
                  FROM res_users AS delegator
                  JOIN res_company AS company ON company.id = delegator.company_id
                 WHERE legacy_grant.user_id = delegator.id
                """
            )
            cr.execute(
                """
                SELECT COUNT(*)
                  FROM pdp_delegation_grant AS legacy_grant
                  LEFT JOIN res_users AS delegator ON delegator.id = legacy_grant.user_id
                  LEFT JOIN res_company AS company ON company.id = delegator.company_id
                 WHERE company.currency_id IS NULL
                """
            )
            if cr.fetchone()[0]:
                raise RuntimeError(
                    "Cannot migrate delegation grant currency: delegator company currency is unavailable."
                )
        return result

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
            grant._max_amount_minor_units()
            issued_at = _unix_seconds(grant.valid_from)
            valid_until = _unix_seconds(grant.valid_until)
            if valid_until <= issued_at:
                raise UserError(_("Delegation expiry must be after its start time."))
            if valid_until - issued_at > MAX_DELEGATION_TTL_SECONDS:
                raise UserError(_("Delegation validity cannot exceed 24 hours."))
            load_delegation_keyring()
            grant.write({"state": "active"})

    def _max_amount_minor_units(self):
        self.ensure_one()
        if not self.currency_id or not self.currency_id.active:
            raise UserError(_("Delegation grant currency must be active and configured."))
        try:
            return amount_to_minor_units(
                Decimal(str(self.max_amount)), self.currency_id.decimal_places
            )
        except (ValueError, TypeError) as exc:
            raise UserError(
                _("Delegation maximum must be exact at the selected currency precision.")
            ) from exc

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

    def build_protected_intent(self, order, command_id):
        """Build the V2 proof from trusted persisted Odoo purchase-order state."""
        self.ensure_one()
        if self.state != "active":
            raise UserError(_("The selected delegation grant is not active."))
        if not order.ai_agent_id or order.ai_agent_id != self.agent_id:
            raise UserError(_("The purchase order agent does not match the delegation grant."))
        intent = build_purchase_order_intent(order, self, command_id)
        if (
            intent["currency_code"] != self.currency_id.name
            or intent["currency_scale"] != self.currency_id.decimal_places
        ):
            raise AccessError(
                _("Purchase-order currency is outside this delegation grant's currency scope.")
            )
        if intent["amount_minor"] > self._max_amount_minor_units():
            raise AccessError(
                _("Purchase-order amount exceeds this delegation grant's maximum.")
            )
        issued_at = _unix_seconds(self.valid_from)
        valid_until = _unix_seconds(self.valid_until)
        key_id, keys = load_delegation_keyring()
        proof = sign_delegation_proof_v2(
            intent, key_id, keys[key_id], issued_at, valid_until
        )
        return (
            intent,
            proof,
            canonical_business_intent_hash(intent),
            issued_at,
            valid_until,
        )
