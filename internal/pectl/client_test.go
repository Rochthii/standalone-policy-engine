package pectl_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"standalone-policy-engine/internal/pectl"
)

func TestClient_GetSuccess(t *testing.T) {
	expected := map[string]string{"key": "value"}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong Authorization header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(expected)
	}))
	defer ts.Close()

	client := pectl.NewClient(ts.URL, "test-token", 5*time.Second)

	var out map[string]string
	err := client.Request(context.Background(), http.MethodGet, "/test", nil, &out)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if out["key"] != "value" {
		t.Errorf("expected value='value', got: %s", out["key"])
	}
}

func TestClient_PostWithPayload(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode body: %v", err)
		}
		if body["field"] != "val" {
			t.Errorf("unexpected field value: %s", body["field"])
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"abc"}`))
	}))
	defer ts.Close()

	client := pectl.NewClient(ts.URL, "", 5*time.Second)
	payload := map[string]string{"field": "val"}

	var out map[string]string
	err := client.Request(context.Background(), http.MethodPost, "/test", payload, &out)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if out["id"] != "abc" {
		t.Errorf("expected id='abc', got: %s", out["id"])
	}
}

func TestClient_4xxNoRetry(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"title":"Not Found","detail":"policy not found","status":404}`))
	}))
	defer ts.Close()

	client := pectl.NewClient(ts.URL, "", 5*time.Second)
	err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("expected error for 404 response")
	}
	// Should not retry 4xx
	if callCount != 1 {
		t.Errorf("expected 1 call (no retry), got %d", callCount)
	}
}

func TestClient_5xxRetries(t *testing.T) {
	callCount := 0
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"title":"Internal Error","status":500}`))
	}))
	defer ts.Close()

	client := pectl.NewClient(ts.URL, "", 5*time.Second)
	err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("expected error after retries")
	}
	// maxRetries=3 means 1 initial + 3 retries = 4 calls total
	if callCount != 4 {
		t.Errorf("expected 4 calls (1 + 3 retries), got %d", callCount)
	}
}

func TestClient_APIError_ParsedCorrectly(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"type": "https://policy-engine/errors/invalid",
			"title": "Validation Error",
			"status": 400,
			"detail": "policy_text field is required",
			"invalid_params": [{"name": "policy_text", "reason": "must not be empty"}]
		}`))
	}))
	defer ts.Close()

	client := pectl.NewClient(ts.URL, "", 5*time.Second)
	err := client.Request(context.Background(), http.MethodPost, "/test", nil, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*pectl.APIError)
	if !ok {
		t.Fatalf("expected *pectl.APIError, got %T: %v", err, err)
	}
	if apiErr.Status != 400 {
		t.Errorf("expected status 400, got %d", apiErr.Status)
	}
	if apiErr.Title != "Validation Error" {
		t.Errorf("expected 'Validation Error', got: %s", apiErr.Title)
	}
	if len(apiErr.InvalidParams) != 1 {
		t.Errorf("expected 1 invalid param, got %d", len(apiErr.InvalidParams))
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	client := pectl.NewClient(ts.URL, "", 5*time.Second)
	err := client.Request(ctx, http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("expected error due to context cancellation")
	}
}

func TestClient_InvalidBaseURL(t *testing.T) {
	client := pectl.NewClient("http://127.0.0.1:1", "", 1*time.Second)
	err := client.Request(context.Background(), http.MethodGet, "/test", nil, nil)
	if err == nil {
		t.Error("expected error for unreachable server")
	}
}
