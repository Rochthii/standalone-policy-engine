package security

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

// DefaultMasterSecret là khóa bí mật mặc định dùng trong môi trường dev/test khi chưa set biến môi trường.
const DefaultMasterSecret = "pdp_master_secret_key_32bytes!"

// DelegationManager quản lý danh sách thu hồi trên RAM và xác thực chữ ký số ủy quyền HMAC-SHA256.
type DelegationManager struct {
	masterSecret  []byte
	revocationMap sync.Map // grantID (string) -> revokedAt (int64)
}

// NewDelegationManager khởi tạo một DelegationManager mới.
func NewDelegationManager() *DelegationManager {
	secret := os.Getenv("PDP_SHARED_SECRET")
	if secret == "" {
		secret = DefaultMasterSecret
	}
	return &DelegationManager{
		masterSecret: []byte(secret),
	}
}

// Revoke ghi nhận một grantID vào danh sách thu hồi trên RAM (O(1) in-memory lookup).
func (m *DelegationManager) Revoke(grantID string) int64 {
	now := time.Now().Unix()
	m.revocationMap.Store(grantID, now)
	return now
}

// IsRevoked kiểm tra xem grantID đã bị thu hồi hay chưa với độ trễ nano-giây.
func (m *DelegationManager) IsRevoked(grantID string) bool {
	if grantID == "" {
		return false
	}
	_, revoked := m.revocationMap.Load(grantID)
	return revoked
}

// ClearRevocations xóa sạch blacklist (chủ yếu dùng cho test isolation).
func (m *DelegationManager) ClearRevocations() {
	m.revocationMap.Range(func(key, value interface{}) bool {
		m.revocationMap.Delete(key)
		return true
	})
}

// BuildCanonicalString xây dựng chuỗi nối chuẩn hóa theo công thức:
// Payload = grant_id | delegator | agent | amount | valid_until
func BuildCanonicalString(grantID, delegator, agent, amount, validUntil string) string {
	return fmt.Sprintf("%s|%s|%s|%s|%s", grantID, delegator, agent, amount, validUntil)
}

// GenerateProof tạo mã băm HMAC-SHA256 phục vụ test hoặc client wrapper.
func (m *DelegationManager) GenerateProof(grantID, delegator, agent, amount, validUntil string) string {
	payload := BuildCanonicalString(grantID, delegator, agent, amount, validUntil)
	h := hmac.New(sha256.New, m.masterSecret)
	h.Write([]byte(payload))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyProof xác thực chữ ký HMAC của phiên ủy quyền và kiểm tra thời hạn TTL.
// Quy tắc an toàn:
// 1. Phải parse được validUntil sang epoch timestamp int64.
// 2. Thời điểm hiện tại không được vượt quá validUntil (Fail-Closed).
// 3. Chữ ký HMAC phải khớp tuyệt đối (so sánh an toàn qua hmac.Equal).
func (m *DelegationManager) VerifyProof(grantID, delegator, agent, amount, validUntil, proof string) bool {
	if proof == "" || validUntil == "" {
		return false
	}

	// 1. Kiểm tra hết hạn TTL
	expTimestamp, err := strconv.ParseInt(validUntil, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix() > expTimestamp {
		return false // Token đã quá hạn
	}

	// 2. Dựng lại chuỗi Canonical Payload
	payload := BuildCanonicalString(grantID, delegator, agent, amount, validUntil)

	// 3. Tính toán và so khớp HMAC-SHA256
	h := hmac.New(sha256.New, m.masterSecret)
	h.Write([]byte(payload))
	expectedHex := hex.EncodeToString(h.Sum(nil))

	return hmac.Equal([]byte(expectedHex), []byte(proof))
}
