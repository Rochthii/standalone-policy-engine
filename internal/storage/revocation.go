package storage

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"standalone-policy-engine/internal/security"
)

const revocationChannel = "delegation_revocations"

// PersistRevocation atomically stores the revocation and publishes it after
// commit. Repeated revokes never shorten the existing revocation window.
func (s *Storage) PersistRevocation(ctx context.Context, record security.RevocationRecord) (security.RevocationRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return security.RevocationRecord{}, err
	}
	defer tx.Rollback(ctx)

	const query = `INSERT INTO delegation_revocations
        (tenant_id, grant_id, revoked_by, revoked_at, expires_at)
        VALUES ($1, $2, $3, $4, $5)
        ON CONFLICT (tenant_id, grant_id) DO UPDATE SET
            revoked_by = EXCLUDED.revoked_by,
            revoked_at = LEAST(delegation_revocations.revoked_at, EXCLUDED.revoked_at),
            expires_at = GREATEST(delegation_revocations.expires_at, EXCLUDED.expires_at)
        RETURNING tenant_id, grant_id, revoked_by, revoked_at, expires_at;`
	var persisted security.RevocationRecord
	err = tx.QueryRow(ctx, query,
		record.TenantID,
		record.GrantID,
		record.RevokedBy,
		record.RevokedAt,
		record.ExpiresAt,
	).Scan(
		&persisted.TenantID,
		&persisted.GrantID,
		&persisted.RevokedBy,
		&persisted.RevokedAt,
		&persisted.ExpiresAt,
	)
	if err != nil {
		return security.RevocationRecord{}, fmt.Errorf("persist delegation revocation: %w", err)
	}
	payload, err := json.Marshal(persisted)
	if err != nil {
		return security.RevocationRecord{}, fmt.Errorf("encode delegation revocation: %w", err)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_notify($1, $2);`, revocationChannel, string(payload)); err != nil {
		return security.RevocationRecord{}, fmt.Errorf("notify delegation revocation: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return security.RevocationRecord{}, fmt.Errorf("commit delegation revocation: %w", err)
	}
	return persisted, nil
}

// WatchRevocations establishes LISTEN before loading the active snapshot.
// Commits racing with the snapshot are therefore either present in the query,
// queued as notifications, or both; duplicate delivery is idempotent.
func (s *Storage) WatchRevocations(
	ctx context.Context,
	onSnapshot func([]security.RevocationRecord),
	onEvent func(security.RevocationRecord),
) error {
	conn, err := s.pool.Acquire(ctx)
	if err != nil {
		return fmt.Errorf("acquire revocation listener: %w", err)
	}
	defer conn.Release()

	if _, err := conn.Exec(ctx, "LISTEN "+revocationChannel); err != nil {
		return fmt.Errorf("listen for delegation revocations: %w", err)
	}
	rows, err := conn.Query(ctx, `SELECT tenant_id, grant_id, revoked_by, revoked_at, expires_at
        FROM delegation_revocations WHERE expires_at > CURRENT_TIMESTAMP;`)
	if err != nil {
		return fmt.Errorf("load active delegation revocations: %w", err)
	}
	records := make([]security.RevocationRecord, 0)
	for rows.Next() {
		var record security.RevocationRecord
		if err := rows.Scan(&record.TenantID, &record.GrantID, &record.RevokedBy, &record.RevokedAt, &record.ExpiresAt); err != nil {
			rows.Close()
			return fmt.Errorf("scan delegation revocation: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return fmt.Errorf("iterate delegation revocations: %w", err)
	}
	rows.Close()
	onSnapshot(records)
	log.Printf("[Storage] loaded %d active revocations and listening on %s", len(records), revocationChannel)

	for {
		notification, err := conn.Conn().WaitForNotification(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("wait for delegation revocation: %w", err)
		}
		var record security.RevocationRecord
		if err := json.Unmarshal([]byte(notification.Payload), &record); err != nil {
			log.Printf("[Storage] ignoring malformed revocation event: %v", err)
			continue
		}
		onEvent(record)
	}
}
