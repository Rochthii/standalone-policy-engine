package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"standalone-policy-engine/internal/engine"
)

type healthDatabaseStub struct{ err error }

func (s healthDatabaseStub) Ping(context.Context) error { return s.err }

type healthSyncStub struct {
	status engine.SyncStatus
	err    error
}

func (s healthSyncStub) Readiness(context.Context) (engine.SyncStatus, error) {
	return s.status, s.err
}

func TestReadinessEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		database   healthDatabaseStub
		syncer     healthSyncStub
		wantCode   int
		wantStatus string
	}{
		{
			name:       "healthy",
			syncer:     healthSyncStub{status: engine.SyncStatusHealthy},
			wantCode:   http.StatusOK,
			wantStatus: "healthy",
		},
		{
			name:       "degraded sync leaves endpoints",
			syncer:     healthSyncStub{status: engine.SyncStatusDegraded, err: errors.New("listener disconnected")},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "degraded",
		},
		{
			name:       "initial sync is not ready",
			syncer:     healthSyncStub{status: engine.SyncStatusNotReady, err: errors.New("sync is starting")},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "not_ready",
		},
		{
			name:       "postgres unavailable is not ready",
			database:   healthDatabaseStub{err: errors.New("connection refused")},
			syncer:     healthSyncStub{status: engine.SyncStatusHealthy},
			wantCode:   http.StatusServiceUnavailable,
			wantStatus: "not_ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			registerHealthEndpoints(mux, tt.database, tt.syncer)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/readyz", nil))
			if recorder.Code != tt.wantCode {
				t.Fatalf("status code = %d, want %d", recorder.Code, tt.wantCode)
			}
			var body healthResponse
			if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			if body.Status != tt.wantStatus {
				t.Fatalf("status = %q, want %q", body.Status, tt.wantStatus)
			}
		})
	}
}

func TestLivenessEndpointDoesNotDependOnPostgreSQLOrSync(t *testing.T) {
	mux := http.NewServeMux()
	registerHealthEndpoints(mux, healthDatabaseStub{err: errors.New("down")}, healthSyncStub{status: engine.SyncStatusDegraded, err: errors.New("down")})
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/livez", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("liveness status code = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestHealthAliasUsesReadinessContract(t *testing.T) {
	mux := http.NewServeMux()
	registerHealthEndpoints(mux, healthDatabaseStub{}, healthSyncStub{status: engine.SyncStatusHealthy})
	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/health", nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("health alias status code = %d, want %d", recorder.Code, http.StatusOK)
	}
}
