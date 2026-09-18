# Active Task

ID: AUD-ARCHIVE-02
Goal: Add a configurable immutable audit-archive delivery boundary
Scope: internal/audit, internal/config, PDP server wiring, AWS archive IaC
Acceptance: archive segments use unique immutable keys; disabled mode preserves current delivery; IaC encodes Object Lock controls
Validation: go test ./internal/audit ./internal/config ./cmd/pdp-server; go vet ./internal/audit ./internal/config ./cmd/pdp-server; git diff --check
Commit: d37f232
