"""CanonicalBusinessIntent v1 primitives for the bounded Odoo PO workflow."""

import base64
import hashlib
import re
import struct
from datetime import datetime
from decimal import Decimal, InvalidOperation


INT64_MAX = (1 << 63) - 1
INT64_MIN = -(1 << 63)
INTENT_VERSION = "cbi.v1"
PROOF_VERSION = "v2"
RESOURCE_TYPE = "purchase.order"
ACTION = "action:CONFIRM_PURCHASE_ORDER"
INTENT_DOMAIN = b"PDP-CANONICAL-BUSINESS-INTENT"
STATE_DOMAIN = b"PDP-ODOO-PO-STATE-V1"

CBI_FIELDS = (
    "intent_version",
    "tenant_id",
    "company_id",
    "resource_type",
    "resource_id",
    "action",
    "vendor_id",
    "currency_code",
    "currency_scale",
    "amount_minor",
    "line_digest",
    "record_state",
    "record_write_version",
    "state_witness",
    "creator_subject",
    "delegation_grant_id",
    "delegator_subject",
    "agent_subject",
    "command_id",
    "proof_version",
)

_MONEY = re.compile(r"^(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$")
_DIGEST = re.compile(r"^[0-9a-f]{64}$")
_CURRENCY = re.compile(r"^[A-Z]{3}$")
_TIMESTAMP = re.compile(r"^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}\.[0-9]{6}Z$")
_COMMAND_ID = re.compile(r"^[A-Za-z0-9_-]{43}$")


def amount_to_minor_units(amount, currency_scale):
    """Convert an exact decimal amount without binary floats or rounding."""
    scale = _integer("currency_scale", currency_scale, minimum=0, maximum=6)
    if isinstance(amount, bool) or isinstance(amount, float):
        raise ValueError("amount must not use a binary float")
    if isinstance(amount, int):
        text = str(amount)
    elif isinstance(amount, (str, Decimal)):
        text = str(amount)
    else:
        raise ValueError("amount must be an integer, Decimal or plain decimal string")
    if not _MONEY.fullmatch(text):
        raise ValueError("amount must be a non-negative plain decimal without exponent")
    try:
        exact = Decimal(text)
    except InvalidOperation as exc:
        raise ValueError("amount is not a valid exact decimal") from exc
    scaled = exact * (Decimal(10) ** scale)
    if scaled != scaled.to_integral_value():
        raise ValueError("amount has precision beyond the trusted currency scale")
    minor = int(scaled)
    if minor > INT64_MAX:
        raise ValueError("amount_minor exceeds signed 64-bit range")
    return minor


def with_state_witness(values):
    """Return a copy with the material-state witness freshly computed."""
    candidate = dict(values)
    allowed = set(CBI_FIELDS)
    if set(candidate) not in (allowed, allowed - {"state_witness"}):
        raise ValueError("CBI fields are missing or unknown")
    candidate.pop("state_witness", None)
    _validate_fields(candidate, require_witness=False)
    candidate["state_witness"] = hashlib.sha256(_state_bytes(candidate)).hexdigest()
    return candidate


def canonical_business_intent_bytes(values):
    """Validate and encode CBI v1 in the same ordinal format as Go."""
    _validate_fields(values, require_witness=True)
    expected = hashlib.sha256(_state_bytes(values)).hexdigest()
    if values["state_witness"] != expected:
        raise ValueError("state_witness does not match material state")

    payload = bytearray(INTENT_DOMAIN)
    payload.extend(_string(values["intent_version"]))
    payload.extend(_string(values["tenant_id"]))
    payload.extend(_int64(values["company_id"]))
    payload.extend(_string(values["resource_type"]))
    payload.extend(_int64(values["resource_id"]))
    payload.extend(_string(values["action"]))
    payload.extend(_int64(values["vendor_id"]))
    payload.extend(_string(values["currency_code"]))
    payload.extend(_int64(values["currency_scale"]))
    payload.extend(_int64(values["amount_minor"]))
    payload.extend(bytes.fromhex(values["line_digest"]))
    payload.extend(_string(values["record_state"]))
    payload.extend(_string(values["record_write_version"]))
    payload.extend(bytes.fromhex(values["state_witness"]))
    payload.extend(_string(values["creator_subject"]))
    payload.extend(_int64(values["delegation_grant_id"]))
    payload.extend(_string(values["delegator_subject"]))
    payload.extend(_string(values["agent_subject"]))
    payload.extend(_string(values["command_id"]))
    payload.extend(_string(values["proof_version"]))
    return bytes(payload)


def canonical_business_intent_hash(values):
    return hashlib.sha256(canonical_business_intent_bytes(values)).hexdigest()


def canonical_business_intent_context(values):
    """Validate CBI and encode its fields for the protobuf string map."""
    canonical_business_intent_bytes(values)
    return {"cbi.%s" % field: str(values[field]) for field in CBI_FIELDS}


def minor_units_to_decimal(amount_minor, currency_scale):
    """Render exact minor units as a canonical non-exponent major-unit decimal."""
    minor = _integer("amount_minor", amount_minor, minimum=0)
    scale = _integer("currency_scale", currency_scale, minimum=0, maximum=6)
    if scale == 0:
        return str(minor)
    digits = str(minor).zfill(scale + 1)
    whole, fraction = digits[:-scale], digits[-scale:].rstrip("0")
    return whole if not fraction else "%s.%s" % (whole, fraction)


def _state_bytes(values):
    payload = bytearray(STATE_DOMAIN)
    payload.extend(_string(values["tenant_id"]))
    payload.extend(_int64(values["company_id"]))
    payload.extend(_string(values["resource_type"]))
    payload.extend(_int64(values["resource_id"]))
    payload.extend(_int64(values["vendor_id"]))
    payload.extend(_string(values["currency_code"]))
    payload.extend(_int64(values["currency_scale"]))
    payload.extend(_int64(values["amount_minor"]))
    payload.extend(bytes.fromhex(values["line_digest"]))
    payload.extend(_string(values["record_state"]))
    payload.extend(_string(values["record_write_version"]))
    payload.extend(_int64(values["delegation_grant_id"]))
    payload.extend(_string(values["creator_subject"]))
    return bytes(payload)


def _validate_fields(values, require_witness):
    expected = set(CBI_FIELDS)
    if not require_witness:
        expected.remove("state_witness")
    if not isinstance(values, dict) or set(values) != expected:
        raise ValueError("CBI fields are missing or unknown")
    if values["intent_version"] != INTENT_VERSION:
        raise ValueError("unsupported canonical business intent version")
    _text("tenant_id", values["tenant_id"])
    for field in ("company_id", "resource_id", "vendor_id", "delegation_grant_id"):
        _integer(field, values[field], minimum=1)
    if values["resource_type"] != RESOURCE_TYPE:
        raise ValueError("unsupported canonical resource type")
    if values["action"] != ACTION:
        raise ValueError("unsupported canonical action")
    if not isinstance(values["currency_code"], str) or not _CURRENCY.fullmatch(values["currency_code"]):
        raise ValueError("currency_code must be three uppercase ASCII letters")
    _integer("currency_scale", values["currency_scale"], minimum=0, maximum=6)
    _integer("amount_minor", values["amount_minor"], minimum=0)
    _digest("line_digest", values["line_digest"])
    _text("record_state", values["record_state"])
    _timestamp("record_write_version", values["record_write_version"])
    if require_witness:
        _digest("state_witness", values["state_witness"])
    _subject("creator_subject", values["creator_subject"], "user:")
    _subject("delegator_subject", values["delegator_subject"], "user:")
    _subject("agent_subject", values["agent_subject"], "agent:")
    _command(values["command_id"])
    if values["proof_version"] != PROOF_VERSION:
        raise ValueError("unsupported canonical proof version")


def _string(value):
    encoded = value.encode("utf-8")
    if len(encoded) > 0xFFFFFFFF:
        raise ValueError("canonical string exceeds uint32 length")
    return struct.pack(">I", len(encoded)) + encoded


def _int64(value):
    return struct.pack(">q", value)


def _integer(field, value, minimum=INT64_MIN, maximum=INT64_MAX):
    if type(value) is not int or value < minimum or value > maximum:
        raise ValueError("%s is outside its canonical integer range" % field)
    return value


def _text(field, value):
    if not isinstance(value, str) or not value or value.strip() != value:
        raise ValueError("%s must be non-empty canonical UTF-8" % field)
    try:
        value.encode("utf-8")
    except UnicodeEncodeError as exc:
        raise ValueError("%s must be non-empty canonical UTF-8" % field) from exc


def _digest(field, value):
    if not isinstance(value, str) or not _DIGEST.fullmatch(value):
        raise ValueError("%s must be lowercase SHA-256 hex" % field)


def _subject(field, value, prefix):
    _text(field, value)
    if not value.startswith(prefix) or len(value) == len(prefix):
        raise ValueError("%s must use %s subject prefix" % (field, prefix))


def _timestamp(field, value):
    if not isinstance(value, str) or not _TIMESTAMP.fullmatch(value):
        raise ValueError("%s must be UTC with exactly six fractional digits" % field)
    try:
        datetime.strptime(value, "%Y-%m-%dT%H:%M:%S.%fZ")
    except ValueError as exc:
        raise ValueError("%s must be UTC with exactly six fractional digits" % field) from exc


def _command(value):
    if not isinstance(value, str) or not _COMMAND_ID.fullmatch(value):
        raise ValueError("command_id must be canonical unpadded base64url for 32 bytes")
    decoded = base64.urlsafe_b64decode(value + "=")
    canonical = base64.urlsafe_b64encode(decoded).rstrip(b"=").decode("ascii")
    if len(decoded) != 32 or canonical != value:
        raise ValueError("command_id must be canonical unpadded base64url for 32 bytes")
