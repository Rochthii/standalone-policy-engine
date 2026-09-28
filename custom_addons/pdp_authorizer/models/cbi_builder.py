"""Trusted Odoo/PostgreSQL reconstruction for CanonicalBusinessIntent v1."""

from datetime import timezone
from decimal import Decimal, InvalidOperation

from odoo.exceptions import AccessError

from ..cbi_lines import canonical_purchase_order_line_digest
from ..cbi_protocol import (
    ACTION,
    INTENT_VERSION,
    PROOF_VERSION,
    RESOURCE_TYPE,
    amount_to_minor_units,
    with_state_witness,
)


def build_purchase_order_intent(order, grant, command_id):
    """Re-read persisted protected fields while holding the purchase-order row lock."""
    order.ensure_one()
    grant.ensure_one()
    order.flush_recordset(
        [
            "company_id",
            "partner_id",
            "currency_id",
            "amount_total",
            "state",
            "delegation_grant_id",
            "delegated_by_id",
            "ai_agent_id",
            "pdp_delegation_nonce",
        ]
    )
    # Stored related/computed fields (notably line.state) may be recomputed at
    # commit and update line.write_date. Drain them before signing the digest,
    # otherwise an unchanged pending order becomes stale across transactions.
    order.order_line.flush_recordset()
    order.env.cr.execute(
        """
        SELECT po.company_id,
               company.pdp_tenant_id,
               po.partner_id,
               currency.name,
               currency.decimal_places,
               po.amount_total::text,
               po.state,
               po.write_date,
               creator.login,
               po.delegation_grant_id,
               po.delegated_by_id,
               po.ai_agent_id
          FROM purchase_order AS po
          JOIN res_company AS company ON company.id = po.company_id
          JOIN res_currency AS currency ON currency.id = po.currency_id
          JOIN res_users AS creator ON creator.id = po.create_uid
         WHERE po.id = %s
         FOR UPDATE OF po
        """,
        [order.id],
    )
    snapshot = order.env.cr.fetchone()
    if not snapshot:
        raise AccessError("Purchase order disappeared during CBI reconstruction.")
    (
        company_id,
        tenant_id,
        vendor_id,
        currency_code,
        currency_scale,
        amount_total,
        record_state,
        record_write_version,
        creator_login,
        grant_id,
        delegated_by_id,
        agent_subject,
    ) = snapshot
    if (
        grant_id != grant.id
        or delegated_by_id != grant.user_id.id
        or agent_subject != grant.agent_id
        or company_id != grant.user_id.company_id.id
        or (tenant_id or "").strip() != grant._tenant_id()
    ):
        raise AccessError("Purchase order and delegation grant identity do not match.")

    lines = _persisted_lines(order)
    intent = {
        "intent_version": INTENT_VERSION,
        "tenant_id": (tenant_id or "").strip(),
        "company_id": company_id,
        "resource_type": RESOURCE_TYPE,
        "resource_id": order.id,
        "action": ACTION,
        "vendor_id": vendor_id,
        "currency_code": currency_code or "",
        "currency_scale": currency_scale,
        "amount_minor": amount_to_minor_units(amount_total, currency_scale),
        "line_digest": canonical_purchase_order_line_digest(lines),
        "record_state": record_state or "",
        "record_write_version": _utc_timestamp(record_write_version),
        "creator_subject": "user:%s" % (creator_login or ""),
        "delegation_grant_id": grant.id,
        "delegator_subject": "user:%s" % (grant.user_id.login or ""),
        "agent_subject": grant.agent_id,
        "command_id": command_id,
        "proof_version": PROOF_VERSION,
    }
    return with_state_witness(intent)


def _persisted_lines(order):
    order.env.cr.execute(
        """
        SELECT id,
               sequence,
               COALESCE(display_type, ''),
               COALESCE(product_id, 0),
               name,
               COALESCE(product_uom, 0),
               product_qty::text,
               price_unit::text,
               date_planned,
               write_date
         FROM purchase_order_line
         WHERE order_id = %s
         ORDER BY sequence, id
         FOR UPDATE
        """,
        [order.id],
    )
    rows = order.env.cr.fetchall()
    line_ids = [row[0] for row in rows]
    records = order.env["purchase.order.line"].sudo().browse(line_ids)
    records.invalidate_recordset(["taxes_id"])
    by_id = {line.id: line for line in records}
    if set(by_id) != set(line_ids):
        raise AccessError("Purchase-order line snapshot is inconsistent.")
    return [
        {
            "line_id": row[0],
            "sequence": row[1],
            "display_type": row[2],
            "product_id": row[3],
            "description": row[4],
            "uom_id": row[5],
            "quantity": _canonical_decimal(row[6]),
            "unit_price": _canonical_decimal(row[7]),
            "tax_ids": by_id[row[0]].taxes_id.ids,
            "planned_at": _utc_timestamp(row[8]) if row[8] else "",
            "line_write_version": _utc_timestamp(row[9]),
        }
        for row in rows
    ]


def _canonical_decimal(value):
    try:
        number = Decimal(str(value))
    except (InvalidOperation, ValueError) as exc:
        raise AccessError("Persisted purchase-order decimal is invalid.") from exc
    if not number.is_finite():
        raise AccessError("Persisted purchase-order decimal is not finite.")
    text = format(number, "f")
    if "." in text:
        text = text.rstrip("0").rstrip(".")
    return "0" if text in ("-0", "") else text


def _utc_timestamp(value):
    if value is None:
        raise AccessError("Persisted purchase-order timestamp is missing.")
    if value.tzinfo is not None:
        value = value.astimezone(timezone.utc).replace(tzinfo=None)
    return value.strftime("%Y-%m-%dT%H:%M:%S.%fZ")
