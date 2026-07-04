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

	// AuditLogsSpilledTotal đếm số lượng audit log bị kích hoạt cơ chế Spill-to-Disk do DB nghẽn.
	AuditLogsSpilledTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "audit_logs_spilled_total",
		Help: "Tổng số lượng audit log bị ghi tạm thời xuống SSD vật lý do PostgreSQL bị nghẽn kết nối.",
	}, []string{"tenant_id"})
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
