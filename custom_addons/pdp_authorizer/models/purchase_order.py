import math
import secrets

from psycopg2 import IntegrityError

from odoo import _, api, fields, models
from odoo.exceptions import AccessError, UserError

from ..pdp_protocol import issue_jwt_from_environment
from .pdp_client import get_pdp_client


class PurchaseOrder(models.Model):
    _inherit = "purchase.order"

    pdp_status = fields.Selection(
        [
            ("pending", "Pending"),
            ("allow", "Allowed"),
            ("require_approval", "Human approval required"),
            ("deny", "Denied"),
        ],
        default="pending",
        readonly=True,
        copy=False,
    )
    ai_agent_id = fields.Char(copy=False)
    delegated_by_id = fields.Many2one("res.users", copy=False)
    delegation_grant_id = fields.Many2one("pdp.delegation.grant", copy=False)
    pdp_delegation_nonce = fields.Char(readonly=True, copy=False)
    pdp_department = fields.Char(required=True, default="Procurement")

    @api.onchange("delegation_grant_id")
    def _onchange_delegation_grant_id(self):
        if self.delegation_grant_id:
            self.ai_agent_id = self.delegation_grant_id.agent_id
            self.delegated_by_id = self.delegation_grant_id.user_id

    def _tenant_id(self):
        self.ensure_one()
        tenant_id = (self.company_id.pdp_tenant_id or "").strip()
        if not tenant_id:
            raise UserError(_("The purchase-order company has no PDP Tenant ID configured."))
        return tenant_id

    def _identity_attributes(self, subject):
        if subject.startswith("agent:"):
            return {}
        return {"department": self.env.user.pdp_department}

    def _base_request(self, subject):
        self.ensure_one()
        creator = "user:%s" % self.create_uid.login
        return {
            "subject": subject,
            "action": "action:APPROVE_PURCHASE_ORDER",
            "resource": "purchase_order:%s" % self.id,
            "context": {
                "amount": str(int(math.ceil(self.amount_total))),
                "currency": self.currency_id.name or "",
                "resource.creator_id": creator,
                "resource.department": self.pdp_department,
                "branch": self.company_id.city or "",
                "delegation_chain": subject,
            },
        }

    def _authorization_attempt(self, grant, nonce, fingerprint):
        self.ensure_one()
        model = self.env["pdp.authorization.attempt"].sudo()
        domain = [
            ("tenant_id", "=", self._tenant_id()),
            ("delegation_grant_id", "=", grant.id),
            ("delegation_nonce", "=", nonce),
        ]
        existing = model.search(domain, limit=1)
        if existing:
            return self._validate_existing_attempt(existing, fingerprint), False
        try:
            with self.env.cr.savepoint():
                attempt = model.create(
                    {
                        "tenant_id": self._tenant_id(),
                        "delegation_grant_id": grant.id,
                        "delegation_nonce": nonce,
                        "request_fingerprint": fingerprint,
                        "business_model": self._name,
                        "business_res_id": self.id,
                        "valid_until": grant.valid_until,
                    }
                )
                attempt.flush_recordset()
            return attempt, True
        except IntegrityError:
            existing = model.search(domain, limit=1)
            if not existing:
                raise AccessError(_("Concurrent PDP authorization attempt could not be resolved."))
            return self._validate_existing_attempt(existing, fingerprint), False

    def _validate_existing_attempt(self, attempt, fingerprint):
        self.ensure_one()
        if (
            attempt.request_fingerprint != fingerprint
            or attempt.business_model != self._name
            or attempt.business_res_id != self.id
        ):
            raise AccessError(_("Delegation nonce was replayed with a different business command."))
        if attempt.state == "pending":
            raise AccessError(_("A prior authorization attempt is still pending."))
        return attempt

    def _delegated_request(self):
        self.ensure_one()
        grant = self.delegation_grant_id
        if grant.user_id.company_id != self.company_id:
            raise UserError(_("Delegation grant and purchase order belong to different companies."))
        nonce = self._locked_delegation_nonce()
        values, proof, fingerprint = grant.build_protected_tuple(self, nonce)
        attempt, created = self._authorization_attempt(grant, nonce, fingerprint)
        request = self._base_request(values["agent"])
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
        return request, attempt, created

    def _locked_delegation_nonce(self):
        """Serialize nonce creation so concurrent confirms reuse one command ID."""
        self.ensure_one()
        self.env.cr.execute(
            "SELECT pdp_delegation_nonce FROM purchase_order WHERE id = %s FOR UPDATE",
            [self.id],
        )
        row = self.env.cr.fetchone()
        if not row:
            raise AccessError(_("Purchase order disappeared during PDP authorization."))
        nonce = row[0]
        if nonce:
            self.invalidate_recordset(["pdp_delegation_nonce"])
            return nonce
        nonce = secrets.token_urlsafe(32)
        self.write({"pdp_delegation_nonce": nonce})
        return nonce

    def _schedule_pdp_activity(self, obligation, advice):
        self.ensure_one()
        approver = self.delegated_by_id or self.env.ref(
            "base.user_admin", raise_if_not_found=False
        ) or self.env.user
        note = obligation.get("message") or advice.get("reason") or _(
            "The PDP requires human approval for this purchase order."
        )
        self.activity_schedule(
            "mail.mail_activity_data_todo",
            summary=_("PDP human approval required: %s") % self.name,
            note=note,
            user_id=approver.id,
        )

    def _pdp_confirm_one(self):
        self.ensure_one()
        if self.delegation_grant_id:
            request, attempt, created = self._delegated_request()
            if not created:
                return True
        else:
            subject = "user:%s" % self.env.user.login
            request = self._base_request(subject)
            attempt = None

        tenant_id = self._tenant_id()
        token = issue_jwt_from_environment(
            request["subject"],
            tenant_id,
            attributes=self._identity_attributes(request["subject"]),
        )
        decision, obligations, advice = get_pdp_client().check_access(
            tenant_id,
            request["subject"],
            request["action"],
            request["resource"],
            request["context"],
            token,
        )
        unsupported = [
            item["type"]
            for item in obligations
            if item["type"] != "REQUIRE_HUMAN_APPROVAL"
        ]
        if unsupported:
            raise AccessError(_("The PDP returned an obligation unsupported by this PEP."))
        if decision == "ALLOW":
            self.write({"pdp_status": "allow"})
            result = super(PurchaseOrder, self).button_confirm()
            if attempt:
                attempt.mark_completed("executed", "allow", self.state)
            return result

        approval = next(
            (item for item in obligations if item["type"] == "REQUIRE_HUMAN_APPROVAL"),
            None,
        )
        if approval:
            self.write({"state": "to approve", "pdp_status": "require_approval"})
            self._schedule_pdp_activity(approval, advice)
            if attempt:
                attempt.mark_completed("approval_required", "deny", self.state)
            return True

        self.write({"pdp_status": "deny"})
        raise AccessError(_("The PDP denied this purchase-order confirmation."))

    def button_confirm(self):
        for order in self:
            order._pdp_confirm_one()
        return True
