"""Initialize only the isolated ABC database; run real committed comparisons."""
import argparse
import importlib.util
import os
from pathlib import Path
import subprocess
import sys

DATABASE = "odoo_eval_abc"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--reuse", action="store_true", help="Reuse an initialized evaluation DB")
    args = parser.parse_args()
    if os.environ.get("PDP_EVAL_ONLY") != DATABASE:
        raise RuntimeError("ABC runner requires the isolated evaluation Compose overlay")
    spec = importlib.util.spec_from_file_location("e2e_setup", Path(__file__).with_name("run-odoo-e2e.py"))
    setup = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(setup)
    setup.TEST_DATABASE = DATABASE
    if not args.reuse:
        # Deliberately fail if it exists: never silently delete a database.
        connection = setup.database_connection("postgres")
        connection.autocommit = True
        try:
            with connection.cursor() as cursor:
                cursor.execute("CREATE DATABASE odoo_eval_abc OWNER pdp_admin")
        finally:
            connection.close()
        setup.seed_pdp()
        connection = setup.database_connection("policy_engine")
        try:
            with connection:
                with connection.cursor() as cursor:
                    cursor.execute("""INSERT INTO policies (id,tenant_id,effect,policy_text,status,version)
                        VALUES ('10000000-0000-0000-0000-000000000005',%s,'FORBID',%s,'ACTIVE',1)""",
                        [os.environ["PDP_TENANT_ID"], 'forbid(principal == any, '
                         'action == action:CONFIRM_PURCHASE_ORDER, resource == any) '
                         'when { resource.department == "Blocked" };'])
                    cursor.execute("UPDATE tenants SET revision=revision+1 WHERE id=%s",
                                   [os.environ["PDP_TENANT_ID"]])
        finally:
            connection.close()
        subprocess.run([
            "odoo", "--database=" + DATABASE, "--db-filter=^odoo_eval_abc$",
            "--db_host=" + os.environ["HOST"], "--db_user=" + os.environ["USER"],
            "--db_password=" + os.environ["PASSWORD"], "--init=pdp_authorizer",
            "--stop-after-init", "--without-demo=all", "--log-level=warn",
        ], check=True)
    setup.wait_for_pdp()
    setup.verify_mtls_boundary()
    subprocess.run([sys.executable, str(Path(__file__).with_name("odoo_abc_compare.py"))], check=True)


if __name__ == "__main__":
    main()
