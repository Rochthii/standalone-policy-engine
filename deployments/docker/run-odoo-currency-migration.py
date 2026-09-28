"""Exercise the required grant-currency field upgrade on a disposable database."""

import os
import subprocess
from datetime import timedelta

import psycopg2
from psycopg2 import sql
import odoo
from odoo import SUPERUSER_ID, Command, api, fields


DATABASE = "odoo_currency_migration"
LEGACY_MAXIMA = {"USD": "1234.56", "EUR": "987.65"}


def connect(database):
    return psycopg2.connect(
        host=os.environ.get("PDP_DB_HOST", "testbed-db"),
        port=int(os.environ.get("PDP_DB_PORT", "5432")),
        user=os.environ["USER"],
        password=os.environ["PASSWORD"],
        dbname=database,
    )


def recreate_database():
    if DATABASE in {"odoo", "odoo_e2e", os.environ.get("PDP_TESTBED_ERP_DATABASE")}:
        raise RuntimeError("currency migration test database must be dedicated")
    connection = connect("postgres")
    connection.autocommit = True
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                "SELECT pg_terminate_backend(pid) FROM pg_stat_activity "
                "WHERE datname = %s AND pid <> pg_backend_pid()",
                (DATABASE,),
            )
            cursor.execute(sql.SQL("DROP DATABASE IF EXISTS {}").format(sql.Identifier(DATABASE)))
            cursor.execute(
                sql.SQL("CREATE DATABASE {} OWNER {}").format(
                    sql.Identifier(DATABASE), sql.Identifier(os.environ["USER"])
                )
            )
    finally:
        connection.close()


def run_odoo(module_flag):
    subprocess.run(
        [
            "odoo",
            "--database=" + DATABASE,
            "--db-filter=^%s$" % DATABASE,
            "--db_host=" + os.environ["HOST"],
            "--db_port=" + os.environ.get("PDP_DB_PORT", "5432"),
            "--db_user=" + os.environ["USER"],
            "--db_password=" + os.environ["PASSWORD"],
            module_flag,
            "--stop-after-init",
            "--without-demo=all",
            "--log-level=warn",
        ],
        check=True,
    )


def create_legacy_grants():
    odoo.tools.config.parse_config(
        [
            "--database=" + DATABASE,
            "--db_host=" + os.environ["HOST"],
            "--db_port=" + os.environ.get("PDP_DB_PORT", "5432"),
            "--db_user=" + os.environ["USER"],
            "--db_password=" + os.environ["PASSWORD"],
        ]
    )
    registry = odoo.registry(DATABASE)
    with registry.cursor() as cursor:
        env = api.Environment(cursor, SUPERUSER_ID, {})
        eur = env.ref("base.EUR")
        eur.active = True
        eur_company = env["res.company"].create(
            {"name": "Migration EUR Company", "currency_id": eur.id}
        )
        admin = env["res.users"].browse(2)
        usd_company = admin.company_id
        eur_user = env["res.users"].with_context(no_reset_password=True).create(
            {
                "name": "Migration EUR Delegator",
                "login": "currency_migration_eur_delegator",
                "company_id": eur_company.id,
                "company_ids": [Command.set([eur_company.id])],
                "groups_id": [Command.set([env.ref("base.group_user").id])],
            }
        )
        now = fields.Datetime.now()
        grant_model = env["pdp.delegation.grant"]
        usd_grant = grant_model.create(
            {
                "user_id": admin.id,
                "currency_id": usd_company.currency_id.id,
                "agent_id": "agent:migration-usd",
                "max_amount": LEGACY_MAXIMA["USD"],
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
                "state": "active",
            }
        )
        eur_grant = grant_model.create(
            {
                "user_id": eur_user.id,
                "currency_id": eur.id,
                "agent_id": "agent:migration-eur",
                "max_amount": LEGACY_MAXIMA["EUR"],
                "valid_from": now - timedelta(minutes=1),
                "valid_until": now + timedelta(minutes=10),
                "state": "active",
            }
        )
        grant_ids = {"USD": usd_grant.id, "EUR": eur_grant.id}
        cursor.commit()

    connection = connect(DATABASE)
    try:
        with connection:
            with connection.cursor() as cursor:
                # Reproduce the pre-currency schema but retain existing grants.
                cursor.execute(
                    "ALTER TABLE pdp_delegation_grant DROP COLUMN currency_id"
                )
                cursor.execute(
                    "UPDATE ir_module_module SET latest_version = '17.0.5.0.0' "
                    "WHERE name = 'pdp_authorizer'"
                )
    finally:
        connection.close()
    return grant_ids


def verify_upgrade(grant_ids):
    connection = connect(DATABASE)
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT grant_row.max_amount, grant_row.state, grant_row.currency_id,
                       currency.name, company.currency_id
                  FROM pdp_delegation_grant AS grant_row
                  JOIN res_users AS delegator ON delegator.id = grant_row.user_id
                  JOIN res_company AS company ON company.id = delegator.company_id
                  JOIN res_currency AS currency ON currency.id = grant_row.currency_id
                 WHERE grant_row.id = ANY(%s)
                 ORDER BY grant_row.id
                """,
                (list(grant_ids.values()),),
            )
            rows = cursor.fetchall()
            if len(rows) != len(grant_ids):
                raise AssertionError("one or more legacy grant rows disappeared")
            for grant_id, row in zip(sorted(grant_ids.values()), rows):
                amount, state, currency_id, currency_name, company_currency_id = row
                expected_code = next(
                    code for code, expected_id in grant_ids.items()
                    if expected_id == grant_id
                )
                if str(amount) != LEGACY_MAXIMA[expected_code] or state != "active":
                    raise AssertionError(
                        "upgrade changed legacy grant amount/state: %r" % (row,)
                    )
                if currency_id != company_currency_id:
                    raise AssertionError(
                        "legacy %s grant currency %s does not match delegator company currency %s"
                        % (expected_code, currency_name, company_currency_id)
                    )
            cursor.execute(
                "SELECT latest_version FROM ir_module_module WHERE name = 'pdp_authorizer'"
            )
            if cursor.fetchone()[0] != "17.0.6.0.0":
                raise AssertionError("module version did not advance to the migration release")
            print(
                "ODOO-CURRENCY-MIGRATION PASS: USD and EUR legacy grants retained exact maxima and company currencies",
                flush=True,
            )
    finally:
        connection.close()


def verify_later_update_preserves_explicit_currency(grant_id):
    connection = connect(DATABASE)
    try:
        with connection:
            with connection.cursor() as cursor:
                cursor.execute(
                    "SELECT id FROM res_currency WHERE name = 'EUR' AND active"
                )
                eur_id = cursor.fetchone()[0]
                cursor.execute(
                    "UPDATE pdp_delegation_grant SET currency_id = %s WHERE id = %s",
                    (eur_id, grant_id),
                )
    finally:
        connection.close()
    run_odoo("--update=pdp_authorizer")
    connection = connect(DATABASE)
    try:
        with connection.cursor() as cursor:
            cursor.execute(
                """
                SELECT grant_row.max_amount, grant_row.state, currency.name
                  FROM pdp_delegation_grant AS grant_row
                  JOIN res_currency AS currency ON currency.id = grant_row.currency_id
                 WHERE grant_row.id = %s
                """,
                (grant_id,),
            )
            row = cursor.fetchone()
            if (
                not row
                or str(row[0]) != LEGACY_MAXIMA["USD"]
                or row[1:] != ("active", "EUR")
            ):
                raise AssertionError(
                    "subsequent module update changed an explicit grant currency: %r"
                    % (row,)
                )
            print(
                "ODOO-CURRENCY-MIGRATION PASS: later module upgrade preserves explicit currency override",
                flush=True,
            )
    finally:
        connection.close()


if __name__ == "__main__":
    recreate_database()
    run_odoo("--init=pdp_authorizer")
    legacy_grant_ids = create_legacy_grants()
    run_odoo("--update=pdp_authorizer")
    verify_upgrade(legacy_grant_ids)
    verify_later_update_preserves_explicit_currency(legacy_grant_ids["USD"])
