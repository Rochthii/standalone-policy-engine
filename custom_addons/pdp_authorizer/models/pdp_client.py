import logging
import os
import threading

import grpc
from odoo import _
from odoo.exceptions import AccessError
from v1 import policy_pb2, policy_pb2_grpc


try:
    import gevent.monkey
    from grpc.experimental import gevent as grpc_gevent

    if gevent.monkey.is_module_patched("socket"):
        grpc_gevent.init_gevent()
except ImportError:
    pass


_logger = logging.getLogger(__name__)


def _read_binary(path):
    with open(path, "rb") as source:
        return source.read()


class SafePDPClient:
    """PID-aware standard-Protobuf client with fail-closed authentication."""

    _lock = threading.Lock()

    def __init__(self, target=None, timeout=0.35):
        self._target = target or os.environ.get("PDP_GRPC_TARGET", "localhost:50051")
        self._timeout = timeout
        self._pid = os.getpid()
        self._channel = None
        self._stub = None
        self._init_channel()

    def _channel_credentials(self):
        ca_path = os.environ.get("PDP_CLIENT_TLS_CA", "").strip()
        cert_path = os.environ.get("PDP_CLIENT_TLS_CERT", "").strip()
        key_path = os.environ.get("PDP_CLIENT_TLS_KEY", "").strip()
        configured = [bool(ca_path), bool(cert_path), bool(key_path)]
        production = os.environ.get("PDP_CLIENT_ENV", "").lower() == "production"
        if any(configured) and not all(configured):
            raise RuntimeError("PDP client mTLS CA, certificate and key must be configured together")
        if not all(configured):
            if production:
                raise RuntimeError("PDP client mTLS is mandatory in production")
            return None
        return grpc.ssl_channel_credentials(
            root_certificates=_read_binary(ca_path),
            private_key=_read_binary(key_path),
            certificate_chain=_read_binary(cert_path),
        )

    def _init_channel(self):
        if self._channel is not None:
            self._channel.close()
        options = [
            ("grpc.keepalive_time_ms", 10000),
            ("grpc.keepalive_timeout_ms", 2000),
            ("grpc.keepalive_permit_without_calls", True),
            ("grpc.http2.max_pings_without_data", 0),
        ]
        server_name = os.environ.get("PDP_TLS_SERVER_NAME", "").strip()
        if server_name:
            options.append(("grpc.ssl_target_name_override", server_name))
        credentials = self._channel_credentials()
        if credentials is None:
            _logger.warning("PDP client is using non-production insecure transport")
            self._channel = grpc.insecure_channel(self._target, options=options)
        else:
            self._channel = grpc.secure_channel(
                self._target, credentials, options=options
            )
        self._stub = policy_pb2_grpc.PolicyDecisionPointStub(self._channel)

    def _get_stub(self):
        current_pid = os.getpid()
        if current_pid != self._pid:
            with self._lock:
                if current_pid != self._pid:
                    self._pid = current_pid
                    self._init_channel()
        return self._stub

    @staticmethod
    def _metadata(token):
        if not token:
            raise AccessError(_("PDP authentication token is missing."))
        return (("authorization", "Bearer " + token),)

    def check_access(self, tenant_id, subject, action, resource, context, token):
        request = policy_pb2.CheckAccessRequest(
            tenant_id=str(tenant_id),
            subject=str(subject),
            action=str(action),
            resource=str(resource),
            context={str(key): str(value) for key, value in (context or {}).items()},
        )
        try:
            response = self._get_stub().CheckAccess(
                request, timeout=self._timeout, metadata=self._metadata(token)
            )
        except grpc.RpcError as exc:
            _logger.error("PDP CheckAccess failed closed with status %s", exc.code())
            raise AccessError(_("PDP authorization is unavailable; transaction denied.")) from exc
        decision = (
            "ALLOW"
            if response.decision == policy_pb2.CheckAccessResponse.ALLOW
            else "DENY"
        )
        obligations = [
            {
                "type": item.type,
                "message": item.message,
                "payload": dict(item.payload),
            }
            for item in response.obligations
        ]
        return decision, obligations, dict(response.advice)

    def revoke_delegation(self, tenant_id, grant_id, revoked_by, token, reason=""):
        request = policy_pb2.RevokeRequest(
            tenant_id=str(tenant_id),
            grant_id=str(grant_id),
            revoked_by=str(revoked_by),
            reason=str(reason),
        )
        try:
            response = self._get_stub().RevokeDelegation(
                request, timeout=self._timeout, metadata=self._metadata(token)
            )
        except grpc.RpcError as exc:
            _logger.error("PDP RevokeDelegation failed closed with status %s", exc.code())
            raise AccessError(_("Delegation revocation was not accepted by the PDP.")) from exc
        if not response.success:
            raise AccessError(_("Delegation revocation was rejected by the PDP."))
        return response


_client = None
_client_lock = threading.Lock()


def get_pdp_client():
    global _client
    if _client is None:
        with _client_lock:
            if _client is None:
                _client = SafePDPClient()
    return _client
