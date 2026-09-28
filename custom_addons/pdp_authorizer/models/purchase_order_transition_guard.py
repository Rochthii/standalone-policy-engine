"""Guard public Odoo paths that can bypass delegated PO confirmation."""

from odoo import _, api, fields, models
from odoo.exceptions import AccessError

from .purchase_order import (
    _FINAL_TRANSITION_CONTEXT_KEY,
    _FINAL_TRANSITION_SENTINEL,
)


FINAL_STATES = frozenset({"purchase", "done"})
DELEGATION_MARKERS = (
    "delegation_grant_id",
    "ai_agent_id",
    "delegated_by_id",
)


class PurchaseOrderTransitionGuard(models.Model):
    _inherit = "purchase.order"

    pdp_delegated_scope = fields.Boolean(
        default=False,
        readonly=True,
        copy=False,
        index=True,
        help="Sticky marker for purchase orders that entered delegated execution scope.",
    )

    @api.model_create_multi
    def create(self, values_list):
        normalized = []
        for values in values_list:
            values = dict(values)
            if any(values.get(field) for field in DELEGATION_MARKERS):
                values["pdp_delegated_scope"] = True
            normalized.append(values)
        return super().create(normalized)

    def write(self, values):
        values = dict(values)
        values.pop("pdp_delegated_scope", None)
        incoming_scope = any(values.get(field) for field in DELEGATION_MARKERS)
        target_state = values.get("state")
        if target_state in FINAL_STATES:
            self._assert_delegated_final_transition_authorized(incoming_scope)

        legacy_scope = self.filtered(
            lambda order: not order.pdp_delegated_scope
            and any(order[field] for field in DELEGATION_MARKERS)
        )
        clears_marker = any(
            field in values and not values[field] for field in DELEGATION_MARKERS
        )
        if legacy_scope and clears_marker:
            super(PurchaseOrderTransitionGuard, legacy_scope).write(
                {"pdp_delegated_scope": True}
            )
        if incoming_scope:
            values["pdp_delegated_scope"] = True
        return super().write(values)

    def _assert_delegated_final_transition_authorized(self, incoming_scope=False):
        protected = self.filtered(
            lambda order: (
                incoming_scope
                or order.pdp_delegated_scope
                or any(order[field] for field in DELEGATION_MARKERS)
            )
            and order.state not in FINAL_STATES
        )
        if not protected:
            return
        sentinel = self.env.context.get(_FINAL_TRANSITION_CONTEXT_KEY)
        if sentinel is not _FINAL_TRANSITION_SENTINEL:
            raise AccessError(
                _(
                    "Delegated purchase orders must reach a final state through "
                    "the protected confirmation route."
                )
            )
