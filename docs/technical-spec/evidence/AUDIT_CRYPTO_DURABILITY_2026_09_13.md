# AUD-CRYPT-03 Evidence — 2026-09-13

## Verified scope

- Audit subject, action, resource and redacted context are encrypted with a random per-record AES-256-GCM DEK.
- The DEK is wrapped by a versioned active KEK; retained key IDs allow old records and spill files to survive rotation.
- Tenant, decision, request/trace/revision IDs and the exact event timestamp are authenticated as AAD. A keyed integrity tag covers that metadata, wrapped DEK and ciphertext.
- Database failure and queue overflow write encrypted batch files with restricted permissions using temporary-file sync plus atomic rename.
- Restart replay validates each envelope before writing. Stable UUID audit IDs and PostgreSQL `ON CONFLICT DO NOTHING` make crash-after-commit replay idempotent.

## Executed evidence

| Check | Result |
|---|---|
| `go test ./internal/security ./internal/config ./internal/audit ./internal/storage` | PASS |
| `go test -count=1 -v ./internal/storage -run TestStorage_MigrationsIntegration` against PostgreSQL 15 | PASS |
| `go test -count=1 ./...` | PASS |
| `go vet ./...` | PASS |
| `docker compose -f docker-compose.testbed.yml config --quiet` | PASS |
| `git diff --check` | PASS |

The restart/rotation test spills under the old active key, constructs a new logger with the new active key plus the retained old key, replays after restart, and verifies both old and new envelopes. The tamper test changes authenticated tenant metadata and proves replay rejects the file without deleting it. The PostgreSQL integration applies migration `000006`, verifies plaintext request columns are null, decrypts the stored envelope, and inserts the same audit ID again while the row count remains one.

## Explicit limits

- This control detects modifications to retained records; it cannot prove that a privileged storage operator deleted a complete record or spill file.
- External append-only/WORM archival, legal retention policy, KMS/HSM integration and production-path latency/load evidence remain release work.
