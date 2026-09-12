"""Pure protocol helpers shared by the Odoo models and compatibility tests."""

import base64
import hashlib
import hmac
import json
import os
import re
import struct
import time


PROOF_VERSION = "v1"
MAX_DELEGATION_TTL_SECONDS = 24 * 60 * 60
_KEY_ID = re.compile(r"^[A-Za-z0-9_-]{1,64}$")
_STRING_FIELDS = (
    "tenant_id",
    "grant_id",
    "delegator",
    "agent",
    "action",
    "resource",
    "amount",
    "delegation_chain",
    "creator_id",
    "tool_context",
    "execution_mode",
    "nonce",
)
_REQUIRED_FIELDS = (
    "tenant_id",
    "grant_id",
    "delegator",
    "agent",
    "action",
    "resource",
    "delegation_chain",
    "nonce",
)


class PDPConfigurationError(ValueError):
    pass


def _length_prefixed(value):
    encoded = str(value).encode("utf-8")
    return struct.pack(">I", len(encoded)) + encoded


def _validate_tuple(values):
    for field in _REQUIRED_FIELDS:
        if not str(values.get(field, "")).strip():
            raise ValueError("missing delegation field %s" % field)
    issued_at = int(values.get("issued_at", 0))
    valid_until = int(values.get("valid_until", 0))
    if issued_at <= 0 or valid_until <= issued_at:
        raise ValueError("invalid delegation validity window")
    if valid_until - issued_at > MAX_DELEGATION_TTL_SECONDS:
        raise ValueError("delegation TTL exceeds 24 hours")
    if values["delegation_chain"] != "%s,%s" % (
        values["delegator"],
        values["agent"],
    ):
        raise ValueError("delegation chain must bind delegator directly to agent")


def canonical_delegation_bytes(values, key_id):
    """Match internal/security.DelegationProofInput.canonicalBytes exactly."""
    _validate_tuple(values)
    if not _KEY_ID.fullmatch(key_id or ""):
        raise ValueError("invalid delegation key ID")
    payload = bytearray(b"PDP-DELEGATION-PROOF")
    payload.extend(_length_prefixed(PROOF_VERSION))
    payload.extend(_length_prefixed(key_id))
    for field in _STRING_FIELDS:
        payload.extend(_length_prefixed(values.get(field, "")))
    payload.extend(struct.pack(">q", int(values["issued_at"])))
    payload.extend(struct.pack(">q", int(values["valid_until"])))
    return bytes(payload)


def load_delegation_keyring(environment=None):
    if environment is None:
        environment = os.environ
    key_id = environment.get("PDP_DELEGATION_ACTIVE_KID", "").strip()
    raw_keys = environment.get("PDP_DELEGATION_KEYS_JSON", "").strip()
    if not key_id or not raw_keys:
        raise PDPConfigurationError(
            "PDP_DELEGATION_ACTIVE_KID and PDP_DELEGATION_KEYS_JSON are required"
        )
    try:
        keys = json.loads(raw_keys)
    except json.JSONDecodeError as exc:
        raise PDPConfigurationError("invalid PDP delegation key ring JSON") from exc
    if not isinstance(keys, dict) or key_id not in keys:
        raise PDPConfigurationError("active delegation key is not in the key ring")
    if not _KEY_ID.fullmatch(key_id) or not isinstance(keys[key_id], str):
        raise PDPConfigurationError("invalid active delegation key")
    if len(keys[key_id]) < 32:
        raise PDPConfigurationError("active delegation key must contain at least 32 characters")
    return key_id, keys


def sign_delegation_tuple(values, key_id, secret):
    if len(secret or "") < 32:
        raise PDPConfigurationError("delegation signing key is too short")
    payload = canonical_delegation_bytes(values, key_id)
    signature = hmac.new(secret.encode("utf-8"), payload, hashlib.sha256).hexdigest()
    return "%s.%s.%s" % (PROOF_VERSION, key_id, signature)


def delegation_fingerprint(values, key_id):
    return hashlib.sha256(canonical_delegation_bytes(values, key_id)).hexdigest()


def _base64url(raw):
    return base64.urlsafe_b64encode(raw).rstrip(b"=").decode("ascii")


def issue_hs256_jwt(
    subject,
    tenant_id,
    secret,
    issuer,
    audience,
    permissions=(),
    attributes=None,
    now=None,
):
    if len(secret or "") < 32 or not issuer or not audience:
        raise PDPConfigurationError("JWT secret, issuer and audience must be configured")
    now = int(time.time() if now is None else now)
    header = {"alg": "HS256", "typ": "JWT"}
    claims = {
        "sub": subject,
        "tenant_id": tenant_id,
        "iss": issuer,
        "aud": audience,
        "iat": now,
        "exp": now + 60,
    }
    if permissions:
        claims["permissions"] = list(permissions)
    reserved = {"sub", "tenant_id", "iss", "aud", "iat", "exp", "permissions"}
    for key, value in (attributes or {}).items():
        if key not in reserved and value is not None:
            claims[str(key)] = value
    encode = lambda value: _base64url(
        json.dumps(value, separators=(",", ":"), sort_keys=True).encode("utf-8")
    )
    signing_input = "%s.%s" % (encode(header), encode(claims))
    signature = hmac.new(
        secret.encode("utf-8"), signing_input.encode("ascii"), hashlib.sha256
    ).digest()
    return "%s.%s" % (signing_input, _base64url(signature))


def issue_jwt_from_environment(
    subject, tenant_id, permissions=(), attributes=None, environment=None
):
    if environment is None:
        environment = os.environ
    return issue_hs256_jwt(
        subject,
        tenant_id,
        environment.get("PDP_JWT_SECRET", ""),
        environment.get("PDP_JWT_ISSUER", ""),
        environment.get("PDP_JWT_AUDIENCE", ""),
        permissions,
        attributes,
    )
