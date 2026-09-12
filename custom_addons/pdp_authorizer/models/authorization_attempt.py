from odoo import fields, models


class PDPAuthorizationAttempt(models.Model):
    _name = "pdp.authorization.attempt"
    _description = "PDP protected business-command authorization attempt"
    _order = "id desc"

    tenant_id = fields.Char(required=True, index=True, readonly=True)
    delegation_grant_id = fields.Many2one(
        "pdp.delegation.grant", required=True, index=True, readonly=True, ondelete="restrict"
    )
    delegation_nonce = fields.Char(required=True, index=True, readonly=True)
    request_fingerprint = fields.Char(required=True, readonly=True)
    business_model = fields.Char(required=True, readonly=True)
    business_res_id = fields.Integer(required=True, index=True, readonly=True)
    valid_until = fields.Datetime(required=True, index=True, readonly=True)
    state = fields.Selection(
        [
            ("pending", "Pending"),
            ("executed", "Executed"),
            ("approval_required", "Approval required"),
        ],
        required=True,
        default="pending",
        index=True,
        readonly=True,
    )
    decision = fields.Selection(
        [("allow", "Allow"), ("deny", "Deny")], readonly=True
    )
    result_state = fields.Char(readonly=True)
    completed_at = fields.Datetime(readonly=True)

    _sql_constraints = [
        (
            "nonce_scope_uniq",
            "unique(tenant_id, delegation_grant_id, delegation_nonce)",
            "This delegation nonce has already been used for the grant and tenant.",
        )
    ]

    def mark_completed(self, state, decision, result_state):
        self.ensure_one()
        if self.state != "pending":
            return
        self.write(
            {
                "state": state,
                "decision": decision,
                "result_state": result_state,
                "completed_at": fields.Datetime.now(),
            }
        )
