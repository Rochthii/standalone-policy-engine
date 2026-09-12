from odoo import fields, models


class ResUsers(models.Model):
    _inherit = "res.users"

    pdp_department = fields.Char(
        "PDP Department",
        default="Procurement",
        help="Trusted department claim issued by the Odoo PEP for PDP evaluation.",
    )
