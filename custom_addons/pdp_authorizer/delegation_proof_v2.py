"""Delegation proof V2 signing over CanonicalBusinessIntent v1."""

import hashlib
import hmac
import re
import struct

try:
    from .cbi_protocol import canonical_business_intent_hash
except ImportError:  # Direct focused-test loading outside Odoo.
    from cbi_protocol import canonical_business_intent_hash


PROOF_VERSION = "v2"
PROOF_DOMAIN = b"PDP-DELEGATION-PROOF-V2"
MAX_TTL_SECONDS = 24 * 60 * 60
_KEY_ID = re.compile(r"^[A-Za-z0-9_-]{1,64}$")


def canonical_delegation_proof_v2_bytes(intent, key_id, issued_at, valid_until):
    if not _KEY_ID.fullmatch(key_id or ""):
        raise ValueError("invalid V2 delegation key ID")
    if type(issued_at) is not int or type(valid_until) is not int:
        raise ValueError("V2 delegation validity must use signed integers")
    if issued_at <= 0 or valid_until <= issued_at:
        raise ValueError("invalid V2 delegation validity window")
    if valid_until - issued_at > MAX_TTL_SECONDS:
        raise ValueError("V2 delegation TTL exceeds 24 hours")
    intent_hash = bytes.fromhex(canonical_business_intent_hash(intent))
    payload = bytearray(PROOF_DOMAIN)
    payload.extend(_string(PROOF_VERSION))
    payload.extend(_string(key_id))
    payload.extend(intent_hash)
    payload.extend(struct.pack(">q", intent["delegation_grant_id"]))
    payload.extend(_string(intent["agent_subject"]))
    payload.extend(struct.pack(">q", issued_at))
    payload.extend(struct.pack(">q", valid_until))
    return bytes(payload)


def sign_delegation_proof_v2(intent, key_id, secret, issued_at, valid_until):
    if not isinstance(secret, str) or len(secret) < 32:
        raise ValueError("V2 delegation secret must contain at least 32 characters")
    payload = canonical_delegation_proof_v2_bytes(intent, key_id, issued_at, valid_until)
    signature = hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    return "%s.%s.%s" % (PROOF_VERSION, key_id, signature)


def _string(value):
    encoded = value.encode("utf-8")
    return struct.pack(">I", len(encoded)) + encoded
