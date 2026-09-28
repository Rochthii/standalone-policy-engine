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

    def init(self):
        super().init()
        self.env.cr.execute("""
            CREATE OR REPLACE FUNCTION pdp_check_execution_deadline_v1()
            RETURNS trigger LANGUAGE plpgsql AS $$
            DECLARE checked_at timestamp := clock_timestamp() AT TIME ZONE 'UTC';
            BEGIN
                IF NEW.state = 'executed' THEN
                    IF NEW.valid_until <= checked_at OR NOT EXISTS (
                        SELECT 1 FROM pdp_delegation_grant g
                        WHERE g.id = NEW.delegation_grant_id AND g.state = 'active'
                          AND g.valid_from <= checked_at AND g.valid_until > checked_at
                    ) OR EXISTS (
                        SELECT 1 FROM pdp_approval_request a
                        WHERE a.authorization_attempt_id = NEW.id
                          AND (a.state <> 'consumed' OR a.expires_at IS NULL
                               OR a.expires_at <= checked_at)
                    ) THEN
                        RAISE EXCEPTION 'Protected authority expired at deferred commit validation'
                            USING ERRCODE = '23514';
                    END IF;
                END IF;
                RETURN NULL;
            END $$;
            DROP TRIGGER IF EXISTS pdp_execution_deadline ON pdp_authorization_attempt;
            CREATE CONSTRAINT TRIGGER pdp_execution_deadline
            AFTER INSERT OR UPDATE ON pdp_authorization_attempt
            DEFERRABLE INITIALLY DEFERRED FOR EACH ROW
            EXECUTE FUNCTION pdp_check_execution_deadline_v1();
        """)

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
