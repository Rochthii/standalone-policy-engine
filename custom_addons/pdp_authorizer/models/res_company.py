import os

from odoo import fields, models


class ResCompany(models.Model):
    _inherit = "res.company"

    pdp_tenant_id = fields.Char(
        "PDP Tenant ID",
        default=lambda self: os.environ.get("PDP_TENANT_ID", ""),
        copy=False,
        index=True,
        help="Exact tenant identifier provisioned in the standalone PDP.",
    )
