"""Canonical purchase-order line serialization for CBI v1."""

import hashlib
import re
import struct
import unicodedata
from datetime import datetime


LINE_DOMAIN = b"PDP-ODOO-PO-LINES-V1"
LINE_FIELDS = (
    "line_id",
    "sequence",
    "display_type",
    "product_id",
    "description",
    "uom_id",
    "quantity",
    "unit_price",
    "tax_ids",
    "planned_at",
    "line_write_version",
)
INT64_MIN = -(1 << 63)
INT64_MAX = (1 << 63) - 1
_DECIMAL = re.compile(r"^-?(?:0|[1-9][0-9]*)(?:\.[0-9]*[1-9])?$")
_TIMESTAMP = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{6}Z$")


def canonical_purchase_order_lines_bytes(lines):
    if not isinstance(lines, (list, tuple)) or len(lines) > 0xFFFFFFFF:
        raise ValueError("purchase order lines must be a bounded list")
    for line in lines:
        if not isinstance(line, dict) or set(line) != set(LINE_FIELDS):
            raise ValueError("line fields are missing or unknown")
    ordered = sorted(lines, key=lambda line: (line["sequence"], line["line_id"]))
    payload = bytearray(LINE_DOMAIN)
    payload.extend(struct.pack(">I", len(ordered)))
    seen_lines = set()
    for line in ordered:
        line_id = _integer("line_id", line["line_id"], minimum=1)
        if line_id in seen_lines:
            raise ValueError("duplicate purchase order line ID")
        seen_lines.add(line_id)
        payload.extend(_line_bytes(line))
    return bytes(payload)


def canonical_purchase_order_line_digest(lines):
    return hashlib.sha256(canonical_purchase_order_lines_bytes(lines)).hexdigest()


def _line_bytes(line):
    line_id = _integer("line_id", line["line_id"], minimum=1)
    sequence = _integer("sequence", line["sequence"])
    display_type = line["display_type"]
    if display_type not in ("", "line_section", "line_note"):
        raise ValueError("unsupported display_type")
    display_line = display_type != ""
    product_id = _integer("product_id", line["product_id"], minimum=0)
    uom_id = _integer("uom_id", line["uom_id"], minimum=0)
    if (not display_line and (product_id <= 0 or uom_id <= 0)) or (
        display_line and (product_id != 0 or uom_id != 0)
    ):
        raise ValueError("product_id and uom_id do not match display_type")
    description = _description(line["description"])
    quantity = _decimal("quantity", line["quantity"])
    unit_price = _decimal("unit_price", line["unit_price"])
    taxes = _taxes(line["tax_ids"])
    planned_at = line["planned_at"]
    if planned_at == "":
        if not display_line:
            raise ValueError("planned_at is required for product lines")
    else:
        _timestamp("planned_at", planned_at)
    _timestamp("line_write_version", line["line_write_version"])

    payload = bytearray()
    payload.extend(_int64(line_id))
    payload.extend(_int64(sequence))
    payload.extend(_string(display_type))
    payload.extend(_int64(product_id))
    payload.extend(_string(description))
    payload.extend(_int64(uom_id))
    payload.extend(_string(quantity))
    payload.extend(_string(unit_price))
    payload.extend(struct.pack(">I", len(taxes)))
    for tax_id in taxes:
        payload.extend(_int64(tax_id))
    payload.extend(_string(planned_at))
    payload.extend(_string(line["line_write_version"]))
    return bytes(payload)


def _integer(field, value, minimum=INT64_MIN, maximum=INT64_MAX):
    if type(value) is not int or value < minimum or value > maximum:
        raise ValueError("%s is outside its canonical integer range" % field)
    return value


def _string(value):
    if not isinstance(value, str):
        raise ValueError("canonical string must be text")
    encoded = value.encode("utf-8")
    if len(encoded) > 0xFFFFFFFF:
        raise ValueError("canonical string exceeds uint32 length")
    return struct.pack(">I", len(encoded)) + encoded


def _int64(value):
    return struct.pack(">q", value)


def _description(value):
    if not isinstance(value, str):
        raise ValueError("description must be text")
    normalized = unicodedata.normalize("NFC", value.replace("\r\n", "\n").replace("\r", "\n"))
    if not normalized:
        raise ValueError("description must be non-empty canonical UTF-8")
    _string(normalized)
    return normalized


def _decimal(field, value):
    if not isinstance(value, str) or not _DECIMAL.fullmatch(value) or value == "-0":
        raise ValueError("%s must be a canonical decimal" % field)
    return value


def _taxes(values):
    if not isinstance(values, list) or len(values) > 0xFFFFFFFF:
        raise ValueError("tax_ids must be a bounded list")
    taxes = sorted(_integer("tax_id", value, minimum=1) for value in values)
    if len(taxes) != len(set(taxes)):
        raise ValueError("tax_ids must contain unique positive IDs")
    return taxes


def _timestamp(field, value):
    if not isinstance(value, str) or not _TIMESTAMP.fullmatch(value):
        raise ValueError("%s must be UTC with exactly six fractional digits" % field)
    try:
        datetime.strptime(value, "%Y-%m-%dT%H:%M:%S.%fZ")
    except ValueError as exc:
        raise ValueError("%s must be UTC with exactly six fractional digits" % field) from exc
