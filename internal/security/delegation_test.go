package security

import (
	"strconv"
	"sync"
	"testing"
	"time"
)

func TestDelegationManager_VerifyProof_Success(t *testing.T) {
	mgr := NewDelegationManager()

	grantID := "42"
	delegator := "user:bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	validUntil := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	proof := mgr.GenerateProof(grantID, delegator, agent, amount, validUntil)

	if !mgr.VerifyProof(grantID, delegator, agent, amount, validUntil, proof) {
		t.Fatalf("expected proof verification to succeed, but failed")
	}
}

func TestDelegationManager_VerifyProof_TamperedAmount(t *testing.T) {
	mgr := NewDelegationManager()

	grantID := "42"
	delegator := "user:bob"
	agent := "agent:procurement_copilot"
	originalAmount := "1500"
	validUntil := strconv.FormatInt(time.Now().Add(1*time.Hour).Unix(), 10)

	proof := mgr.GenerateProof(grantID, delegator, agent, originalAmount, validUntil)

	// Kẻ tấn công sửa số tiền từ 1500 thành 50000
	tamperedAmount := "50000"
	if mgr.VerifyProof(grantID, delegator, agent, tamperedAmount, validUntil, proof) {
		t.Fatalf("expected tampered amount to be rejected, but it was accepted")
	}
}

func TestDelegationManager_VerifyProof_ExpiredTTL(t *testing.T) {
	mgr := NewDelegationManager()

	grantID := "42"
	delegator := "user:bob"
	agent := "agent:procurement_copilot"
	amount := "1500"
	// Đã hết hạn cách đây 10 giây
	pastTime := strconv.FormatInt(time.Now().Add(-10*time.Second).Unix(), 10)

	proof := mgr.GenerateProof(grantID, delegator, agent, amount, pastTime)

	if mgr.VerifyProof(grantID, delegator, agent, amount, pastTime, proof) {
		t.Fatalf("expected expired proof to be rejected (Fail-Closed), but it passed")
	}
}

func TestDelegationManager_RevocationMap_O1(t *testing.T) {
	mgr := NewDelegationManager()
	grantID := "grant-uuid-101"

	if mgr.IsRevoked(grantID) {
		t.Fatalf("grantID should not be revoked initially")
	}

	revokedAt := mgr.Revoke(grantID)
	if revokedAt <= 0 {
		t.Fatalf("revokedAt timestamp should be greater than 0")
	}

	if !mgr.IsRevoked(grantID) {
		t.Fatalf("grantID should be revoked after calling Revoke()")
	}
}

func TestDelegationManager_ConcurrentRace(t *testing.T) {
	mgr := NewDelegationManager()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			grantID := strconv.Itoa(id)
			mgr.IsRevoked(grantID)
			mgr.Revoke(grantID)
			mgr.IsRevoked(grantID)
		}(i)
	}

	wg.Wait()
}
