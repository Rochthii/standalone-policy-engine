"""Serialize committed grant revocation with this database's protected mutations."""

from odoo import models


class PurchaseOrderRevocationFence(models.Model):
    _inherit = "purchase.order"

    def init(self):
        super().init()
        # Private SQL authority table: no public ORM create/write/unlink route.
        # PDP's configured ERP connection may insert or set revoked=true only.
        self.env.cr.execute("""
            CREATE TABLE IF NOT EXISTS pdp_delegation_fence_v1 (
                tenant_id text NOT NULL,
                grant_id text NOT NULL,
                revoked boolean NOT NULL DEFAULT false,
                PRIMARY KEY (tenant_id, grant_id)
            )
        """)

    def _lock_delegation_revocation_fence(self):
        self.ensure_one()
        key = [self._tenant_id(), str(self.delegation_grant_id.id)]
        self.env.cr.execute("""
            INSERT INTO pdp_delegation_fence_v1 (tenant_id, grant_id, revoked)
            VALUES (%s, %s, false) ON CONFLICT (tenant_id, grant_id) DO NOTHING
        """, key)
        self.env.cr.execute("""
            SELECT revoked FROM pdp_delegation_fence_v1
            WHERE tenant_id = %s AND grant_id = %s FOR UPDATE
        """, key)
        # REPEATABLE READ conflicts propagate to Odoo's transaction retry, never
        # continue with the pre-revocation snapshot. Caller retains the lock
        # through business commit; a savepoint/transaction rollback releases it.
        row = self.env.cr.fetchone()
        return row is not None and row[0] is False
