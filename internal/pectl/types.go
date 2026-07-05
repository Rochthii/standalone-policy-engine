package pectl

import "time"

// Policy represents a policy in the system.
type Policy struct {
	ID         string    `json:"id"`
	TenantID   string    `json:"tenant_id"`
	Effect     string    `json:"effect"`
	PolicyText string    `json:"policy_text"`
	Version    int       `json:"version"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// CreatePolicyReq represents the request to create a policy.
type CreatePolicyReq struct {
	Effect     string `json:"effect"`
	PolicyText string `json:"policy_text"`
}

// CreatePolicyResp represents the response when creating a policy.
type CreatePolicyResp struct {
	PolicyID string `json:"policy_id"`
	Status   string `json:"status"`
}

// UpdatePolicyReq represents the request to update a policy.
type UpdatePolicyReq struct {
	PolicyText string `json:"policy_text"`
}

// UpdatePolicyResp represents the response when updating a policy.
type UpdatePolicyResp struct {
	Status string `json:"status"`
}

// PublishPolicyResp represents the response when publishing a policy.
type PublishPolicyResp struct {
	PolicyID         string    `json:"policy_id"`
	Status           string    `json:"status"`
	PublishedVersion int       `json:"published_version"`
	PublishedAt      time.Time `json:"published_at"`
	SyncEventSent    bool      `json:"sync_event_sent"`
}

// SimulateReq represents the simulation request.
type SimulateReq struct {
	Subject       string            `json:"subject"`
	Action        string            `json:"action"`
	Resource      string            `json:"resource"`
	Context       map[string]string `json:"context"`
	Policies      []SimulatePolicy  `json:"policies"`
	IncludeActive bool              `json:"include_active"`
}

// SimulatePolicy represents a raw policy payload in simulation.
type SimulatePolicy struct {
	ID         string `json:"id"`
	PolicyText string `json:"policy_text"`
}

// SimulateResp represents the simulation response.
type SimulateResp struct {
	SimulatedDecision string    `json:"simulated_decision"`
	Reason            string    `json:"reason"`
	MatchedPolicies   []string  `json:"matched_policies"`
	CompileErrors     []string  `json:"compile_errors"`
	TenantID          string    `json:"tenant_id"`
}

// DecisionReq represents the request to check permission or explain decision.
type DecisionReq struct {
	TenantID string            `json:"tenant_id"`
	Subject  string            `json:"subject"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Context  map[string]string `json:"context"`
}

// DecisionResp represents a simple permission check response.
type DecisionResp struct {
	Decision        string    `json:"decision"`
	MatchedPolicyID string    `json:"matched_policy_id"`
	EvaluatedAt     time.Time `json:"evaluated_at"`
}

// MatchedPolicyMetadata represents matched policy metadata in decision explain.
type MatchedPolicyMetadata struct {
	ID         string `json:"id"`
	Effect     string `json:"effect"`
	PolicyText string `json:"policy_text"`
}

// ExplainResp represents a detailed decision explanation response.
type ExplainResp struct {
	Decision        string                  `json:"decision"`
	FinalReason     string                  `json:"final_reason"`
	MatchedPolicies []MatchedPolicyMetadata `json:"matched_policies"`
	Latency         string                  `json:"latency,omitempty"`
}

// Tenant represents tenant basic details.
type Tenant struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TenantStatus represents the operational status of a tenant.
type TenantStatus struct {
	Active       bool      `json:"active"`
	Evicted      bool      `json:"evicted"`
	MemoryUsage  string    `json:"memory_usage"`
	PolicyCount  int       `json:"policy_count"`
	LastActivity time.Time `json:"last_activity"`
}

// MetricsResp represents telemetry metrics.
type MetricsResp struct {
	DecisionRate float64 `json:"decision_rate"`
	QPS          float64 `json:"qps"`
	LatencyP50   float64 `json:"latency_p50_ms"`
	LatencyP95   float64 `json:"latency_p95_ms"`
	LatencyP99   float64 `json:"latency_p99_ms"`
	MemoryUsage  uint64  `json:"memory_usage_bytes"`
	GCStatistics string  `json:"gc_statistics"`
}

// HealthComponent represents a single component health state.
type HealthComponent struct {
	Status  string `json:"status"` // UP, DOWN, UNKNOWN
	Message string `json:"message,omitempty"`
}

// HealthResp represents health check details.
type HealthResp struct {
	Status       string                     `json:"status"` // UP, DOWN, PARTIAL
	Components   map[string]HealthComponent `json:"components"`
}
