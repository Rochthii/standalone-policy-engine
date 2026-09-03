package server

import (
	"testing"
)

func TestReplayBuffer_BasicFlow(t *testing.T) {
	rb := NewReplayBuffer(5)

	// Thêm 5 sự kiện (v1 -> v5)
	for i := 1; i <= 5; i++ {
		rb.Add(PolicyEvent{
			TenantID: "tenant-a",
			PolicyID: "P-1",
			Action:   "UPDATE",
			Revision: uint64(i),
		})
	}

	// Lấy sự kiện sau revision 2 -> phải trả về v3, v4, v5
	events, ok := rb.GetEventsSince("tenant-a", 2)
	if !ok {
		t.Fatal("Mong đợi GetEventsSince thành công")
	}
	if len(events) != 3 {
		t.Fatalf("Mong đợi 3 sự kiện, thực tế: %d", len(events))
	}
	if events[0].Revision != 3 || events[1].Revision != 4 || events[2].Revision != 5 {
		t.Errorf("Thứ tự hoặc nội dung revision sai: %v", events)
	}
}

func TestReplayBuffer_CompactionDetection(t *testing.T) {
	rb := NewReplayBuffer(3)

	// Thêm 4 sự kiện (v1, v2, v3, v4) -> v1 bị đè mất, oldest là v2
	for i := 1; i <= 4; i++ {
		rb.Add(PolicyEvent{
			TenantID: "tenant-a",
			PolicyID: "P-1",
			Action:   "UPDATE",
			Revision: uint64(i),
		})
	}

	// Hỏi revision 1 (đã bị Compaction xóa) -> phải trả về ok = false
	_, ok := rb.GetEventsSince("tenant-a", 1)
	if ok {
		t.Fatal("Mong đợi ok=false do revision 1 đã bị Compaction xóa khỏi Ring Buffer")
	}

	// Hỏi revision 2 (nằm trong buffer) -> phải thành công và trả về v3, v4
	events, ok := rb.GetEventsSince("tenant-a", 2)
	if !ok {
		t.Fatal("Mong đợi ok=true cho revision 2")
	}
	if len(events) != 2 || events[0].Revision != 3 || events[1].Revision != 4 {
		t.Errorf("Kết quả sự kiện không khớp: %v", events)
	}
}
