"""Approved delegated PO confirmation in one Odoo/PostgreSQL transaction."""

from odoo import fields, models
from odoo.exceptions import AccessError

from ..cbi_protocol import canonical_business_intent_context, minor_units_to_decimal
from ..pdp_protocol import issue_jwt_from_environment
from .pdp_client import get_pdp_client
from .purchase_order import (
    _FINAL_TRANSITION_CONTEXT_KEY,
    _FINAL_TRANSITION_SENTINEL,
)


class PurchaseOrderFinal(models.Model):
    _inherit = "purchase.order"

    def _protected_delegated_request(self, intent, proof, issued_at, valid_until):
        """Build the same proof-bound PDP request at initial and final decisions."""
        self.ensure_one()
        request = {
            "subject": intent["agent_subject"],
            "action": intent["action"],
            "resource": "purchase_order:%s" % intent["resource_id"],
            "context": {
                "amount": minor_units_to_decimal(
                    intent["amount_minor"], intent["currency_scale"]
                ),
                "currency": intent["currency_code"],
                "delegation_grant_id": str(intent["delegation_grant_id"]),
                "delegated_by": intent["delegator_subject"],
                "delegation_issued_at": str(issued_at),
                "delegation_valid_until": str(valid_until),
                "delegation_nonce": intent["command_id"],
                "delegation_chain": "%s,%s"
                % (intent["delegator_subject"], intent["agent_subject"]),
                "delegation_proof": proof,
                "resource.creator_id": intent["creator_subject"],
                "resource.department": self.pdp_department,
                "branch": self.company_id.city or "",
                "tool_context": "tool:auto_confirm_po",
                "execution_mode": "autonomous_run",
                "erp.revocation_fence": "odoo-revocation.v1:" + self.env.cr.dbname,
            },
        }
        request["context"].update(canonical_business_intent_context(intent))
        return request

    def _execute_approved_confirmation(self, attempt):
        """Consume one approved command only with its final PO mutation."""
        self.ensure_one()
        attempt.ensure_one()
        approvals = self.env["pdp.approval.request"].sudo().search(
            [("authorization_attempt_id", "=", attempt.id)], limit=2
        )
        if len(approvals) != 1:
            raise AccessError("Protected command has no unique approval record.")
        approval = approvals.ensure_one()
        if approval.state == "pending":
            return True
        if approval.state != "approved":
            raise AccessError("Protected command approval is not executable.")

        current = approval._prepare_final_intent()
        policy_revisions = []
        if not current or not approval._revalidate_approved_capability(policy_revisions):
            return True

        grant = approval.delegation_grant_id
        signed, proof, _fingerprint, issued_at, valid_until = (
            grant.build_protected_intent(self, approval.command_id)
        )
        if signed != current:
            raise AccessError("Final agent proof does not match the locked approval intent.")
        request = self._protected_delegated_request(
            current, proof, issued_at, valid_until
        )
        token = issue_jwt_from_environment(current["agent_subject"], approval.tenant_id)
        decision, obligations, advice = get_pdp_client().check_access(
            approval.tenant_id,
            current["agent_subject"],
            current["action"],
            request["resource"],
            request["context"],
            token,
        )
        valid_obligation = (
            len(obligations) == 1
            and obligations[0]["type"] == "REQUIRE_HUMAN_APPROVAL"
        )
        if decision != "ALLOW" or (obligations and not valid_obligation):
            raise AccessError("Current agent policy denied approved confirmation.")

        if not self._lock_delegation_revocation_fence():
            approval._set_terminal("invalidated", "delegation_revoked")
            return True

        self._lock_current_authority(policy_revisions + [advice.get("erp.policy_revision")])

        approval.write(
            {
                "state": "consumed",
                "terminal_reason": "executed",
                "terminal_at": fields.Datetime.now(),
            }
        )
        authorized = self.with_context(
            **{_FINAL_TRANSITION_CONTEXT_KEY: _FINAL_TRANSITION_SENTINEL}
        )
        authorized.button_approve()
        self.invalidate_recordset(["state"])
        if self.state not in ("purchase", "done"):
            raise AccessError("Approved confirmation did not reach a final PO state.")
        self.write({"pdp_status": "allow"})
        attempt.write(
            {
                "state": "executed",
                "decision": "allow",
                "result_state": self.state,
                "completed_at": fields.Datetime.now(),
            }
        )
        return True
