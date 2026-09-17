package server

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"time"

	"standalone-policy-engine/internal/engine"
)

const healthTimeout = time.Second

type databaseHealth interface {
	Ping(context.Context) error
}

type syncHealth interface {
	Readiness(context.Context) (engine.SyncStatus, error)
}

type healthComponent struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type healthResponse struct {
	Status     string                     `json:"status"`
	Components map[string]healthComponent `json:"components"`
}

func registerHealthEndpoints(mux *http.ServeMux, database databaseHealth, syncer syncHealth) {
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		writeHealth(w, http.StatusOK, healthResponse{Status: string(engine.SyncStatusHealthy)})
	})
	ready := func(w http.ResponseWriter, r *http.Request) {
		status, response := readiness(r.Context(), database, syncer)
		writeHealth(w, status, response)
	}
	mux.HandleFunc("GET /readyz", ready)
	mux.HandleFunc("GET /api/v1/health", ready)
}

func readiness(parent context.Context, database databaseHealth, syncer syncHealth) (int, healthResponse) {
	ctx, cancel := context.WithTimeout(parent, healthTimeout)
	defer cancel()
	components := make(map[string]healthComponent, 2)
	ready := true
	overall := engine.SyncStatusHealthy

	if database == nil {
		components["postgresql"] = healthComponent{Status: string(engine.SyncStatusNotReady), Message: "storage is not initialized"}
		ready = false
		overall = engine.SyncStatusNotReady
	} else if err := database.Ping(ctx); err != nil {
		components["postgresql"] = healthComponent{Status: string(engine.SyncStatusNotReady), Message: err.Error()}
		ready = false
		overall = engine.SyncStatusNotReady
	} else {
		components["postgresql"] = healthComponent{Status: string(engine.SyncStatusHealthy)}
	}

	if syncer == nil {
		components["policy_sync"] = healthComponent{Status: string(engine.SyncStatusNotReady), Message: "syncer is not initialized"}
		ready = false
		overall = engine.SyncStatusNotReady
	} else if syncStatus, err := syncer.Readiness(ctx); err != nil {
		components["policy_sync"] = healthComponent{Status: string(syncStatus), Message: err.Error()}
		ready = false
		if overall != engine.SyncStatusNotReady {
			overall = syncStatus
		}
	} else {
		components["policy_sync"] = healthComponent{Status: string(engine.SyncStatusHealthy)}
	}

	if !ready {
		return http.StatusServiceUnavailable, healthResponse{Status: string(overall), Components: components}
	}
	return http.StatusOK, healthResponse{Status: string(engine.SyncStatusHealthy), Components: components}
}

func writeHealth(w http.ResponseWriter, status int, response healthResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(response)
}

// StartReadinessServer exposes only liveness and readiness endpoints for the
// PDP data plane. It intentionally does not expose control-plane APIs.
func StartReadinessServer(port int, database databaseHealth, syncer syncHealth) (*http.Server, error) {
	mux := http.NewServeMux()
	registerHealthEndpoints(mux, database, syncer)
	server := &http.Server{Addr: ":" + strconv.Itoa(port), Handler: mux}
	listener, err := net.Listen("tcp", server.Addr)
	if err != nil {
		return nil, err
	}
	go func() { _ = server.Serve(listener) }()
	return server, nil
}
