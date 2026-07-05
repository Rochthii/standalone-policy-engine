# `pectl` Command Reference

`pectl` is a CLI tool designed to administer and interact with the Policy Engine REST Control Plane. It enables administrators to manage policy lifecycle, perform simulations, test access checks, inspect tenant state, and retrieve health and telemetry data.

---

## Global Options

All commands accept these global flags:

- `--config string`     Path to custom config file (default: `~/.pectl/config.yaml`)
- `--server string`     REST Control Plane server URL (default: `http://localhost:8080`)
- `--token string`      JWT authentication token
- `--output string`     Output formatting mode: `table`, `json`, or `yaml` (default: `table`)
- `--timeout string`    Request timeout duration, e.g., `5s`, `10s` (default: `10s`)

---

## Config Priority

Configuration is evaluated in this order (highest priority overrides lower):
1. Command Line Flags
2. Environment Variables (`PECTL_SERVER`, `PECTL_TOKEN`, `PECTL_OUTPUT`)
3. Configuration file (`~/.pectl/config.yaml`)
4. Internal Defaults

---

## Command Reference

### Version
```bash
pectl version
```
Displays CLI version, git commit hash, and compile time.

### Policy Management
Manage policies for tenants. Most commands require tenant authentication.

- **Create a policy** (Status: `DRAFT`)
  ```bash
  pectl policy create <tenant_id> --effect permit|forbid --file <file_path>
  ```
- **Update policy contents** (Resets status to `DRAFT`)
  ```bash
  pectl policy update <tenant_id> <policy_id> --file <file_path>
  ```
- **Publish a policy** (Compiles AST, synchronizes cluster, changes status to `ACTIVE`)
  ```bash
  pectl policy publish <tenant_id> <policy_id>
  ```
- **Delete a policy**
  ```bash
  pectl policy delete <tenant_id> <policy_id>
  ```
- **List policies for a tenant**
  ```bash
  pectl policy list <tenant_id>
  ```
- **Get detailed policy information**
  ```bash
  pectl policy get <tenant_id> <policy_id>
  ```

### Simulation
Run dry-run authorization checks on temporary policies or overrides.

```bash
pectl simulate <tenant_id> \
  --subject <subject> \
  --action <action> \
  --resource <resource> \
  [--context-file <context_json_file>] \
  [--draft-file <draft_cedar_file>] \
  [--include-active]
```

- `--draft-file` specifies a local policy DSL to evaluate.
- `--include-active` merges the draft policy with existing active policies from the tenant.
- `--context-file` loads dynamic attributes context as key-value JSON string pairs.

### Decision Query

- **Check Access**
  ```bash
  pectl check <tenant_id> --subject <subject> --action <action> --resource <resource>
  ```
  Returns `ALLOW` or `DENY` decision with total latency details.

- **Explain Decision**
  ```bash
  pectl explain <tenant_id> --subject <subject> --action <action> --resource <resource>
  ```
  Returns `ALLOW`/`DENY`, the logic reason, and the full trace of matching rules.

### Tenant Commands

- **List Tenants**
  ```bash
  pectl tenant list
  ```
- **Get Tenant Info**
  ```bash
  pectl tenant get <tenant_id>
  ```
- **Inspect Tenant Cache Status**
  ```bash
  pectl tenant status <tenant_id>
  ```
  Retrieves memory consumption, policy counts, and time of last activity.

### System Diagnostics

- **Telemetry Metrics**
  ```bash
  pectl metrics
  ```
  Shows QPS, evaluation decision rates, GC stats, and P50/P95/P99 latency.

- **Health Checks**
  ```bash
  pectl health
  ```
  Verifies the operational status of database, tenant cache, engine, and metric systems.
