package metrics

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Các biến Prometheus metrics toàn cục
var (
	// PolicyEvaluationDuration đo lường độ trễ (latency) của luồng đánh giá quyết định trong RAM.
	PolicyEvaluationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "policy_evaluation_duration_seconds",
		Help:    "Độ trễ thời gian đánh giá chính sách phân quyền trên RAM (giây).",
		Buckets: []float64{0.0001, 0.0002, 0.0005, 0.001, 0.002, 0.005, 0.01}, // Buckets nhỏ từ 0.1ms đến 10ms
	}, []string{"tenant_id", "decision"})

	// RequestTotal đếm tổng số yêu cầu gRPC CheckAccess.
	RequestTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_requests_total",
		Help: "Tổng số lượng yêu cầu phân quyền gRPC nhận được.",
	}, []string{"tenant_id", "decision"})

	// ActivePoliciesCount ghi nhận số lượng chính sách đang hoạt động trên RAM.
	ActivePoliciesCount = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "active_policies_count",
		Help: "Số lượng chính sách đang được nạp trên bộ nhớ RAM của PDP Engine.",
	}, []string{"tenant_id"})

	// AuditLogsStreamedTotal đếm số lượng audit log được stream ra Stdout / UDS.
	AuditLogsStreamedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_logs_streamed_total",
		Help: "Tổng số lượng audit log được stream ra Stdout / UDS.",
	}, []string{"tenant_id"})

	// AuditLogsSpilledTotal đếm số lượng audit log bị kích hoạt cơ chế Spill-to-Disk do DB nghẽn.
	AuditLogsSpilledTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_logs_spilled_total",
		Help: "Tổng số lượng audit log bị ghi tạm thời xuống SSD vật lý do PostgreSQL bị nghẽn kết nối.",
	}, []string{"tenant_id"})

	AuditLogsDroppedTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_logs_dropped_total",
		Help: "Audit records dropped because the bounded queue or durable sink was unavailable.",
	}, []string{"tenant_id"})

	AuditBatchWriteFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audit_batch_write_failures_total",
		Help: "Durable audit batch writes that failed or timed out.",
	})

	AuditLogsReplayedTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audit_logs_replayed_total",
		Help: "Encrypted audit records restored from durable spill files.",
	})

	AuditSpillFailuresTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "audit_spill_failures_total",
		Help: "Audit spill or replay operations that failed, including integrity failures.",
	})
)

// ObserveEvaluationDuration ghi nhận độ trễ thời gian xử lý quyết định.
func ObserveEvaluationDuration(tenantID, decision string, duration time.Duration) {
	PolicyEvaluationDuration.WithLabelValues(tenantID, decision).Observe(duration.Seconds())
}

// IncrementRequestCounter tăng bộ đếm số request phân quyền.
func IncrementRequestCounter(tenantID, decision string) {
	RequestTotal.WithLabelValues(tenantID, decision).Inc()
}

// UpdateActivePoliciesCount cập nhật số lượng chính sách đang lưu trên RAM.
func UpdateActivePoliciesCount(tenantID string, count int) {
	ActivePoliciesCount.WithLabelValues(tenantID).Set(float64(count))
}

// IncrementAuditLogsSpilled tăng bộ đếm log bị ghi đĩa dự phòng.
func IncrementAuditLogsSpilled(tenantID string) {
	AuditLogsSpilledTotal.WithLabelValues(tenantID).Inc()
}

func AddAuditLogsDropped(tenantID string, count uint64) {
	AuditLogsDroppedTotal.WithLabelValues(tenantID).Add(float64(count))
}

func IncrementAuditBatchWriteFailures() {
	AuditBatchWriteFailuresTotal.Inc()
}

func AddAuditLogsReplayed(count uint64) {
	AuditLogsReplayedTotal.Add(float64(count))
}

func IncrementAuditSpillFailures() {
	AuditSpillFailuresTotal.Inc()
}
