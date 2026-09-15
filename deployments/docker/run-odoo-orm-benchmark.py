"""Prepare a fresh Odoo database and run the real ORM/PDP comparison."""

import os
import socket
import subprocess
import time

import psycopg2
from psycopg2 import sql


TEST_DATABASE = "odoo_orm_benchmark"


def database_connection(database):
    return psycopg2.connect(
        host=os.environ.get("PDP_DB_HOST", "testbed-db"),
        port=int(os.environ.get("PDP_DB_PORT", "5432")),
        user=os.environ["USER"],
        password=os.environ["PASSWORD"],
        dbname=database,
    )


def recreate_database():
    connection = database_connection("postgres")
    connection.autocommit = True
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT pg_terminate_backend(pid) FROM pg_stat_activity "
                "WHERE datname = %s AND pid <> pg_backend_pid()",
                (TEST_DATABASE,),
            )
            cursor.execute(sql.SQL("DROP DATABASE IF EXISTS {}").format(sql.Identifier(TEST_DATABASE)))
            cursor.execute(
                sql.SQL("CREATE DATABASE {} OWNER {}").format(
                    sql.Identifier(TEST_DATABASE), sql.Identifier(os.environ["USER"])
                )
            )
    finally:
        connection.close()


def seed_pdp():
    deadline = time.monotonic() + 60
    while True:
        try:
            connection = database_connection("policy_engine")
            with connection:
                with connection.cursor() as cursor:
                    cursor.execute("SELECT to_regclass('public.tenants')")
                    if cursor.fetchone()[0]:
                        with open(os.environ["PDP_E2E_POLICY_SQL"], encoding="utf-8") as source:
                            cursor.execute(source.read())
                        cursor.execute("SELECT id::text FROM tenants WHERE name = 'odoo-e2e'")
                        os.environ["PDP_TENANT_ID"] = cursor.fetchone()[0]
                        return
            connection.close()
        except psycopg2.Error:
            pass
        if time.monotonic() >= deadline:
            raise RuntimeError("PDP schema was not ready before the benchmark deadline")
        time.sleep(1)


def wait_for_pdp():
    host, port = os.environ["PDP_GRPC_TARGET"].rsplit(":", 1)
    deadline = time.monotonic() + 60
    while time.monotonic() < deadline:
        try:
            with socket.create_connection((host, int(port)), timeout=1):
                return
        except OSError:
            time.sleep(1)
    raise RuntimeError("PDP gRPC port was not ready before the benchmark deadline")


def run_benchmark():
    subprocess.run(
        [
            "odoo",
            "--database=" + TEST_DATABASE,
            "--db-filter=^%s$" % TEST_DATABASE,
            "--db_host=" + os.environ["HOST"],
            "--db_port=" + os.environ.get("PDP_DB_PORT", "5432"),
            "--db_user=" + os.environ["USER"],
            "--db_password=" + os.environ["PASSWORD"],
            "--init=pdp_authorizer",
            "--test-enable",
            "--test-tags=pdp_benchmark",
            "--stop-after-init",
            "--without-demo=all",
            "--log-level=test",
        ],
        check=True,
    )


if __name__ == "__main__":
    recreate_database()
    seed_pdp()
    wait_for_pdp()
    run_benchmark()
