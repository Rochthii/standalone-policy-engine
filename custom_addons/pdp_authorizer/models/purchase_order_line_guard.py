"""Serialize delegated line membership/material edits with PO authorization."""

from odoo import api, models


class PurchaseOrderLineGuard(models.Model):
    _inherit = "purchase.order.line"

    def _touch_delegated_parents(self, order_ids):
        # Lock before child writes (including M2M tax changes). A real parent
        # update, not just a lock, makes a stale REPEATABLE READ final retry.
        if not order_ids:
            return
        self.env.cr.execute("""
            SELECT id FROM purchase_order
             WHERE id = ANY(%s)
               AND (pdp_delegated_scope OR delegation_grant_id IS NOT NULL
                    OR delegated_by_id IS NOT NULL OR COALESCE(ai_agent_id, '') <> '')
             ORDER BY id FOR UPDATE
        """, [sorted(set(order_ids))])
        protected = [row[0] for row in self.env.cr.fetchall()]
        if protected:
            self.env.cr.execute("""
                UPDATE purchase_order SET write_date = clock_timestamp() AT TIME ZONE 'UTC'
                 WHERE id = ANY(%s)
            """, [protected])
            self.env["purchase.order"].browse(protected).invalidate_recordset(["write_date"])

    @api.model_create_multi
    def create(self, values_list):
        self.check_access_rights("create")
        default_order = self.default_get(["order_id"]).get("order_id")
        parents = [values.get("order_id", default_order) for values in values_list]
        self._touch_delegated_parents([value for value in parents if value])
        return super().create(values_list)

    def write(self, values):
        self.check_access_rights("write")
        self.check_access_rule("write")
        parents = self.order_id.ids
        if values.get("order_id"):
            parents = parents + [values["order_id"]]
        self._touch_delegated_parents(parents)
        return super().write(values)

    def unlink(self):
        self.check_access_rights("unlink")
        self.check_access_rule("unlink")
        self._touch_delegated_parents(self.order_id.ids)
        return super().unlink()
