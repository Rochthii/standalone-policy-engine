package server

import (
	"sync"
)

// PolicyEvent đại diện cho một sự kiện thay đổi chính sách được lưu trữ trong Replay Buffer.
type PolicyEvent struct {
	TenantID string `json:"tenant_id"`
	PolicyID string `json:"policy_id"`
	Action   string `json:"action"`   // UPDATE hoặc DELETE
	Revision uint64 `json:"revision"` // Số hiệu phiên bản tăng dần đơn điệu
}

// ReplayBuffer là Ring Buffer trong RAM lưu trữ N sự kiện thay đổi chính sách gần nhất.
// Giúp các PDP node bị rớt mạng ngắn hạn có thể Catch-Up ngay lập tức mà không cần query lại toàn bộ DB.
type ReplayBuffer struct {
	mu       sync.RWMutex
	capacity int
	events   []PolicyEvent
	head     int
	count    int
}

// NewReplayBuffer khởi tạo ReplayBuffer với dung lượng tối đa (mặc định 1,000 sự kiện).
func NewReplayBuffer(capacity int) *ReplayBuffer {
	if capacity <= 0 {
		capacity = 1000
	}
	return &ReplayBuffer{
		capacity: capacity,
		events:   make([]PolicyEvent, capacity),
	}
}

// Add thêm một sự kiện mới vào Ring Buffer (ghi đè sự kiện cũ nhất khi đầy).
func (rb *ReplayBuffer) Add(event PolicyEvent) {
	rb.mu.Lock()
	defer rb.mu.Unlock()

	idx := (rb.head + rb.count) % rb.capacity
	if rb.count == rb.capacity {
		rb.head = (rb.head + 1) % rb.capacity
	} else {
		rb.count++
	}
	rb.events[idx] = event
}

// GetEventsSince trả về danh sách các sự kiện kể từ afterRevision.
// Nếu afterRevision quá cũ (đã bị Compaction xóa khỏi Ring Buffer), trả về (nil, false).
func (rb *ReplayBuffer) GetEventsSince(tenantID string, afterRevision uint64) ([]PolicyEvent, bool) {
	rb.mu.RLock()
	defer rb.mu.RUnlock()

	if rb.count == 0 {
		return []PolicyEvent{}, true
	}

	oldestIdx := rb.head
	oldestRev := rb.events[oldestIdx].Revision

	// Nếu afterRevision nhỏ hơn số hiệu cũ nhất trong buffer -> đã bị Compacted!
	if afterRevision > 0 && afterRevision < oldestRev {
		return nil, false // Báo hiệu cần Full Snapshot reload
	}

	result := make([]PolicyEvent, 0)
	for i := 0; i < rb.count; i++ {
		idx := (rb.head + i) % rb.capacity
		ev := rb.events[idx]
		if ev.TenantID == tenantID && ev.Revision > afterRevision {
			result = append(result, ev)
		}
	}
	return result, true
}
