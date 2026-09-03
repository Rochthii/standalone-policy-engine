---
name: docker-standards
description: Expert rules for Docker, pinned testbed environments, UTF-8 BOM prevention, and E2E testing.
---

# Docker & Containerized Engineering Standards Skill

## 🎯 Mission
Prevent dependency drift (2026–2029), eliminate Windows UTF-8 BOM encoding traps, and provide reproducible testbeds.

## 🔑 Critical Hardened Rules
1. **Pinned Base Images & Dependency Drift Prevention**:
   - Never use floating tags like `golang:latest` or `alpine:latest`.
   - Frozen Testbed (`docker-compose.testbed.yml`):
     - `golang:1.22-alpine` for Go PDP and test runner.
     - `odoo:17.0` for ERP core.
     - `postgres:15-alpine` for PostgreSQL.
   - Build statically: `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s"`.

2. **Single-Command Reproducible Testbed**:
   ```bash
   docker compose -f docker-compose.testbed.yml up --abort-on-container-exit
   ```
   Container `testbed-runner` automatically executes all 7 vectors, benchmarks hot-path, and shuts down stack.

3. **Zero UTF-8 BOM Trap on Windows**:
   - PowerShell `Set-Content -Encoding UTF8` inserts a hidden Byte Order Mark (`\ufeff`). PostgreSQL parses this as syntax error, breaking migrations.
   - Standard safe write in PowerShell:
     `[System.IO.File]::WriteAllText($path, $content, (New-Object System.Text.UTF8Encoding($false)))`

4. **Healthcheck & Startup Handshake**:
   - Database: `healthcheck: test: ["CMD-SHELL", "pg_isready -U postgres"]`.
   - Apps: `depends_on: { db: { condition: service_healthy } }`.

## 📂 Source Files
- [`docker-compose.testbed.yml`](file:///e:/Projects/Project_TN/standalone-policy-engine/docker-compose.testbed.yml)
- [`deployments/docker/Dockerfile.pdp`](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.pdp)
- [`deployments/docker/Dockerfile.control`](file:///e:/Projects/Project_TN/standalone-policy-engine/deployments/docker/Dockerfile.control)
- [`docs/technical-spec/RUNBOOK_OPS.md`](file:///e:/Projects/Project_TN/standalone-policy-engine/docs/technical-spec/RUNBOOK_OPS.md)