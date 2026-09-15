#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""Retired synthetic baseline entry point.

The former implementation used ``time.sleep`` and a hardcoded PDP latency. It
was not an empirical measurement and must not be used in reports. Run
``make benchmark-odoo-orm`` for the real Odoo ORM/PostgreSQL versus mTLS gRPC
PDP comparison.
"""

raise SystemExit("Run `make benchmark-odoo-orm`; the synthetic baseline was retired.")
