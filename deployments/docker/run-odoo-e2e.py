"""Prepare isolated databases, seed the live PDP, then execute Odoo tests."""

import os
import socket
import subprocess
import time

import grpc
import psycopg2
from psycopg2 import sql
from v1 import policy_pb2, policy_pb2_grpc


TEST_DATABASE = "odoo_e2e"


def database_connection(database):
    return psycopg2.connect(
        host=os.environ.get("PDP_DB_HOST", "testbed-db"),
        port=int(os.environ.get("PDP_DB_PORT", "5432")),
        user=os.environ["USER"],
        password=os.environ["PASSWORD"],
        dbname=database,
    )


def recreate_odoo_database():
    connection = database_connection("postgres")
    connection.autocommit = True
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT pg_terminate_backend(pid) FROM pg_stat_activity "
                "WHERE datname = %s AND pid <> pg_backend_pid()",
                (TEST_DATABASE,),
            )
            cursor.execute(sql.SQL("DROP DATABASE IF EXISTS {} ").format(sql.Identifier(TEST_DATABASE)))
            cursor.execute(
                sql.SQL("CREATE DATABASE {} OWNER {}").format(
                    sql.Identifier(TEST_DATABASE), sql.Identifier(os.environ["USER"])
                )
            )
    finally:
        connection.close()


def wait_for_pdp_schema():
    deadline = time.monotonic() + 60
    while True:
        try:
            connection = database_connection("policy_engine")
            try:
                with connection.cursor() as cursor:
                    cursor.execute("SELECT to_regclass('public.tenants')")
                    if cursor.fetchone()[0]:
                        return
            finally:
                connection.close()
        except psycopg2.Error:
            pass
        if time.monotonic() >= deadline:
            raise RuntimeError("PDP schema was not ready before the E2E deadline")
        time.sleep(1)


def seed_pdp():
    wait_for_pdp_schema()
    connection = database_connection("policy_engine")
    try:
        with connection:
            with connection.cursor() as cursor:
                with open(os.environ["PDP_E2E_POLICY_SQL"], encoding="utf-8") as source:
                    cursor.execute(source.read())
                cursor.execute("SELECT id::text FROM tenants WHERE name = 'odoo-e2e'")
                os.environ["PDP_TENANT_ID"] = cursor.fetchone()[0]
    finally:
        connection.close()


def wait_for_pdp():
    host, port = os.environ["PDP_GRPC_TARGET"].rsplit(":", 1)
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            with socket.create_connection((host, int(port)), timeout=1):
                return
        except OSError:
            time.sleep(1)
    raise RuntimeError("PDP gRPC port was not ready before the E2E deadline")


def read_binary(environment_key):
    with open(os.environ[environment_key], "rb") as source:
        return source.read()


def verify_mtls_boundary():
    target = os.environ["PDP_GRPC_TARGET"]
    ca = read_binary("PDP_CLIENT_TLS_CA")
    server_name = os.environ["PDP_TLS_SERVER_NAME"]
    options = (("grpc.ssl_target_name_override", server_name),)
    probe_request = policy_pb2.CheckAccessRequest(
        tenant_id="00000000-0000-0000-0000-000000000017",
        subject="user:mtls-probe",
        action="purchase_order:confirm",
        resource="purchase_order:mtls-probe",
    )

    no_client_credentials = grpc.ssl_channel_credentials(root_certificates=ca)
    no_client_channel = grpc.secure_channel(
        target, no_client_credentials, options=options
    )
    try:
        policy_pb2_grpc.PolicyDecisionPointStub(no_client_channel).CheckAccess(
            probe_request, timeout=3
        )
        raise RuntimeError("PDP accepted a TLS connection without a client certificate")
    except grpc.RpcError as error:
        if error.code() != grpc.StatusCode.UNAVAILABLE:
            raise RuntimeError(
                "missing client certificate returned unexpected status %s" % error.code()
            ) from error
    finally:
        no_client_channel.close()

    credentials = grpc.ssl_channel_credentials(
        root_certificates=ca,
        private_key=read_binary("PDP_CLIENT_TLS_KEY"),
        certificate_chain=read_binary("PDP_CLIENT_TLS_CERT"),
    )
    wrong_name_channel = grpc.secure_channel(
        target,
        credentials,
        options=(("grpc.ssl_target_name_override", "wrong.testbed.invalid"),),
    )
    try:
        policy_pb2_grpc.PolicyDecisionPointStub(wrong_name_channel).CheckAccess(
            probe_request, timeout=3
        )
        raise RuntimeError("PDP client accepted a certificate for the wrong server name")
    except grpc.RpcError as error:
        if error.code() != grpc.StatusCode.UNAVAILABLE:
            raise RuntimeError(
                "wrong server name returned unexpected status %s" % error.code()
            ) from error
    finally:
        wrong_name_channel.close()

    channel = grpc.secure_channel(target, credentials, options=options)
    try:
        policy_pb2_grpc.PolicyDecisionPointStub(channel).CheckAccess(
            probe_request, timeout=3
        )
        raise RuntimeError("authenticated mTLS probe bypassed JWT authentication")
    except grpc.RpcError as error:
        if error.code() != grpc.StatusCode.UNAUTHENTICATED:
            raise RuntimeError(
                "valid client certificate did not reach JWT boundary: %s" % error.code()
            ) from error
    finally:
        channel.close()

    print(
        "MTLS-BOUNDARY PASS: missing client cert and wrong hostname rejected; valid Odoo cert reached JWT boundary",
        flush=True,
    )


def run_odoo_tests():
    arguments = [
        "odoo",
        "--database=" + TEST_DATABASE,
        "--db-filter=^%s$" % TEST_DATABASE,
        "--db_host=" + os.environ["HOST"],
        "--db_port=" + os.environ.get("PDP_DB_PORT", "5432"),
        "--db_user=" + os.environ["USER"],
        "--db_password=" + os.environ["PASSWORD"],
        "--init=pdp_authorizer",
        "--test-enable",
        "--test-tags=/pdp_authorizer",
        "--stop-after-init",
        "--without-demo=all",
        "--log-level=test",
    ]
    subprocess.run(arguments, check=True)


def run_concurrency_test():
    subprocess.run(
        ["python3", os.environ["PDP_E2E_CONCURRENCY_RUNNER"]], check=True
    )


if __name__ == "__main__":
    recreate_odoo_database()
    seed_pdp()
    wait_for_pdp()
    verify_mtls_boundary()
    run_odoo_tests()
    run_concurrency_test()
