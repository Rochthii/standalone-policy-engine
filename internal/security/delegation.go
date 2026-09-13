package security

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// DefaultMasterSecret là khóa bí mật mặc định dùng trong môi trường dev/test khi chưa set biến môi trường.
const DefaultMasterSecret = "pdp_master_secret_key_32bytes!"

// DelegationManager quản lý danh sách thu hồi trên RAM và xác thực chữ ký số ủy quyền HMAC-SHA256.
type DelegationManager struct {
	activeKeyID     string
	signingKeys     map[string][]byte
	revocationTTL   time.Duration
	revocationMap   sync.Map // revocationKey -> revocationRecord
	revocationReady atomic.Bool
}

type revocationKey struct {
	tenantID string
	grantID  string
}

type revocationRecord struct {
	revokedAt int64
	expiresAt int64
}

// RevocationRecord is the durable, tenant-scoped representation shared with
// the persistence and replica synchronization layers.
type RevocationRecord struct {
	TenantID  string    `json:"tenant_id"`
	GrantID   string    `json:"grant_id"`
	RevokedBy string    `json:"revoked_by"`
	RevokedAt time.Time `json:"revoked_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// RevocationStore persists revocations and exposes a snapshot-first watch.
// Implementations must establish the watch before loading the snapshot so a
// commit cannot be lost between startup load and LISTEN registration.
type RevocationStore interface {
	PersistRevocation(context.Context, RevocationRecord) (RevocationRecord, error)
	WatchRevocations(context.Context, func([]RevocationRecord), func(RevocationRecord)) error
}

// NewDelegationManager khởi tạo một DelegationManager mới.
func NewDelegationManager() *DelegationManager {
	secret := os.Getenv("PDP_SHARED_SECRET")
	if secret == "" {
		secret = DefaultMasterSecret
	}
	return NewDelegationManagerWithSecret(secret)
}

// NewDelegationManagerWithSecret builds the manager from centralized config.
func NewDelegationManagerWithSecret(secret string) *DelegationManager {
	manager := &DelegationManager{
		activeKeyID:   "legacy",
		signingKeys:   map[string][]byte{"legacy": []byte(secret)},
		revocationTTL: MaxDelegationTTL,
	}
	manager.revocationReady.Store(true)
	return manager
}

// NewDelegationManagerWithKeyring copies a versioned key ring. GenerateProof
// signs with activeKeyID while VerifyProof accepts every explicitly retained
// key, allowing overlap during rotation without a verification outage.
func NewDelegationManagerWithKeyring(activeKeyID string, keys map[string]string) (*DelegationManager, error) {
	if !validDelegationKeyID(activeKeyID) {
		return nil, errors.New("invalid active delegation key ID")
	}
	if len(keys) == 0 {
		return nil, errors.New("delegation key ring is empty")
	}

	copied := make(map[string][]byte, len(keys))
	for keyID, secret := range keys {
		if !validDelegationKeyID(keyID) {
			return nil, fmt.Errorf("invalid delegation key ID %q", keyID)
		}
		if strings.TrimSpace(secret) == "" {
			return nil, fmt.Errorf("delegation key %q is empty", keyID)
		}
		copied[keyID] = []byte(secret)
	}
	if _, exists := copied[activeKeyID]; !exists {
		return nil, fmt.Errorf("active delegation key %q is not in the key ring", activeKeyID)
	}

	manager := &DelegationManager{
		activeKeyID:   activeKeyID,
		signingKeys:   copied,
		revocationTTL: MaxDelegationTTL,
	}
	manager.revocationReady.Store(true)
	return manager, nil
}

func newDelegationManagerWithRevocationTTL(secret string, revocationTTL time.Duration) *DelegationManager {
	manager := NewDelegationManagerWithSecret(secret)
	manager.revocationTTL = revocationTTL
	return manager
}

func validDelegationKeyID(keyID string) bool {
	if len(keyID) == 0 || len(keyID) > 64 {
		return false
	}
	for _, value := range keyID {
		if (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z') ||
			(value >= '0' && value <= '9') || value == '-' || value == '_' {
			continue
		}
		return false
	}
	return true
}

// Revoke records a tenant-scoped grant revocation. Cleanup is kept off the
// authorization hot path and runs on this infrequent write path.
func (m *DelegationManager) Revoke(tenantID, grantID string) int64 {
	now := time.Now()
	m.cleanupExpired(now.UnixNano())
	m.ApplyRevocation(RevocationRecord{
		TenantID:  tenantID,
		GrantID:   grantID,
		RevokedAt: now,
		ExpiresAt: now.Add(m.revocationTTL),
	})
	return now.Unix()
}

// ApplyRevocation merges a durable or remotely delivered revocation without
// allowing an older duplicate event to shorten its effective lifetime.
func (m *DelegationManager) ApplyRevocation(record RevocationRecord) bool {
	if m == nil || record.TenantID == "" || record.GrantID == "" ||
		record.RevokedAt.IsZero() || !record.ExpiresAt.After(time.Now()) {
		return false
	}
	key := revocationKey{tenantID: record.TenantID, grantID: record.GrantID}
	incoming := revocationRecord{revokedAt: record.RevokedAt.Unix(), expiresAt: record.ExpiresAt.UnixNano()}
	for {
		value, exists := m.revocationMap.Load(key)
		if !exists {
			if _, loaded := m.revocationMap.LoadOrStore(key, incoming); !loaded {
				return true
			}
			continue
		}
		current, ok := value.(revocationRecord)
		if !ok {
			return false
		}
		merged := current
		if incoming.revokedAt < merged.revokedAt {
			merged.revokedAt = incoming.revokedAt
		}
		if incoming.expiresAt > merged.expiresAt {
			merged.expiresAt = incoming.expiresAt
		}
		if merged == current || m.revocationMap.CompareAndSwap(key, current, merged) {
			return true
		}
	}
}

func (m *DelegationManager) SetRevocationReady(ready bool) {
	if m != nil {
		m.revocationReady.Store(ready)
	}
}

func (m *DelegationManager) RevocationReady() bool {
	return m != nil && m.revocationReady.Load()
}

// IsRevoked checks the exact tenant + grant tuple in O(1).
func (m *DelegationManager) IsRevoked(tenantID, grantID string) bool {
	if tenantID == "" || grantID == "" {
		return false
	}
	key := revocationKey{tenantID: tenantID, grantID: grantID}
	value, revoked := m.revocationMap.Load(key)
	if !revoked {
		return false
	}
	record := value.(revocationRecord)
	if time.Now().UnixNano() >= record.expiresAt {
		m.revocationMap.CompareAndDelete(key, record)
		return false
	}
	return true
}

func (m *DelegationManager) cleanupExpired(nowUnixNano int64) {
	m.revocationMap.Range(func(key, value interface{}) bool {
		record := value.(revocationRecord)
		if nowUnixNano >= record.expiresAt {
			m.revocationMap.CompareAndDelete(key, record)
		}
		return true
	})
}

// ClearRevocations xóa sạch blacklist (chủ yếu dùng cho test isolation).
func (m *DelegationManager) ClearRevocations() {
	m.revocationMap.Range(func(key, value interface{}) bool {
		m.revocationMap.Delete(key)
		return true
	})
}
