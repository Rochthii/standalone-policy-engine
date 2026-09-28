"""Order policy publication and local authority writes against protected commit."""

from psycopg2 import sql
from odoo import models
from odoo.exceptions import AccessError


class PurchaseOrderAuthorityFence(models.Model):
    _inherit = "purchase.order"

    def init(self):
        super().init()
        self.env.cr.execute("""
            CREATE TABLE IF NOT EXISTS pdp_policy_fence_v1 (
                tenant_id text PRIMARY KEY, revision bigint NOT NULL,
                ready boolean NOT NULL, publication_id text
            );
            CREATE TABLE IF NOT EXISTS pdp_local_authority_fence_v1 (
                id integer PRIMARY KEY CHECK (id = 1), revision bigint NOT NULL
            );
            INSERT INTO pdp_local_authority_fence_v1 VALUES (1, 0)
                ON CONFLICT (id) DO NOTHING;
            CREATE OR REPLACE FUNCTION pdp_touch_local_authority_v1()
            RETURNS trigger LANGUAGE plpgsql AS $$
            BEGIN
                UPDATE pdp_local_authority_fence_v1 SET revision = revision + 1 WHERE id = 1;
                RETURN NULL;
            END $$;
        """)
        # A coarse epoch deliberately serializes the bounded prototype. Relation
        # triggers cover group-side edits and membership insert/delete phantoms.
        for table in (
            "res_users", "res_groups", "res_groups_users_rel",
            "res_groups_implied_rel", "res_company_users_rel",
            "res_company", "pdp_delegation_grant",
        ):
            self.env.cr.execute(sql.SQL("""
                DROP TRIGGER IF EXISTS pdp_authority_changed ON {};
                CREATE TRIGGER pdp_authority_changed
                AFTER INSERT OR UPDATE OR DELETE OR TRUNCATE ON {}
                FOR EACH STATEMENT EXECUTE FUNCTION pdp_touch_local_authority_v1()
            """).format(sql.Identifier(table), sql.Identifier(table)))

    def _lock_current_authority(self, policy_revisions):
        self.ensure_one()
        if not policy_revisions or any(
            not isinstance(value, str) or not value.isascii() or not value.isdecimal()
            for value in policy_revisions
        ):
            raise AccessError("Missing evaluated policy revision.")
        self.env.cr.execute("SELECT revision FROM pdp_local_authority_fence_v1 WHERE id=1 FOR UPDATE")
        if not self.env.cr.fetchone():
            raise AccessError("Local authority fence is unavailable.")
        self.env.cr.execute("""
            SELECT revision, ready FROM pdp_policy_fence_v1
            WHERE tenant_id = %s FOR UPDATE
        """, [self._tenant_id()])
        row = self.env.cr.fetchone()
        if not row or not row[1] or any(int(value) != row[0] for value in policy_revisions):
            raise AccessError("Policy publication is pending or the authorization is stale.")
        # Both row locks survive to commit. A concurrent committed epoch/revision
        # update under Odoo REPEATABLE READ must propagate serialization failure.
