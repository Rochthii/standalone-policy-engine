package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
)

func TestRESTDecisionAuthenticationAndTrustedIdentity(t *testing.T) {
	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	lexer := parser.NewLexer(`permit(principal == any, action == action:READ, resource == file:doc) when { principal.department == "finance" };`)
	p := parser.NewParser(lexer)
	nodes := p.Parse()
	if len(nodes) != 1 || len(p.Errors()) != 0 {
		t.Fatalf("parse REST trusted-identity policy: %v", p.Errors())
	}
	nodes[0].ID = "policy-rest-trusted-identity"
	compiled, err := parser.NewCompiler().Compile(nodes[0])
	if err != nil {
		t.Fatalf("compile REST trusted-identity policy: %v", err)
	}
	if err := eng.UpdateTenantPoliciesWithRevision("tenant-a", []*parser.PolicyNode{compiled}, nil, 1); err != nil {
		t.Fatalf("load REST trusted-identity policy: %v", err)
	}
	mux := NewHTTPServer(nil, eng).ConfigureMux()

	makeToken := func(department string) string {
		claims := jwt.MapClaims{
			"sub":        "user:alice",
			"tenant_id":  "tenant-a",
			"department": department,
			"exp":        time.Now().Add(time.Hour).Unix(),
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		tokenString, signErr := token.SignedString([]byte("test-secret-key-for-sprint-6-unit-test"))
		if signErr != nil {
			t.Fatalf("sign REST test token: %v", signErr)
		}
		return tokenString
	}

	call := func(body, token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/decisions", strings.NewReader(body))
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, req)
		return rr
	}

	t.Run("missing token", func(t *testing.T) {
		rr := call(`{"tenant_id":"tenant-a","subject":"user:alice","action":"READ","resource":"file:doc"}`, "")
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("expected 401, got %d", rr.Code)
		}
	})

	t.Run("body cannot select another tenant", func(t *testing.T) {
		rr := call(`{"tenant_id":"tenant-b","subject":"user:alice","action":"READ","resource":"file:doc"}`, makeToken("finance"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("body cannot select another subject", func(t *testing.T) {
		rr := call(`{"tenant_id":"tenant-a","subject":"user:mallory","action":"READ","resource":"file:doc"}`, makeToken("finance"))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", rr.Code)
		}
	})

	t.Run("request cannot elevate principal attribute", func(t *testing.T) {
		body := `{"tenant_id":"tenant-a","subject":"user:alice","action":"READ","resource":"file:doc","context":{"principal.department":"finance"}}`
		rr := call(body, makeToken("engineering"))
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"decision":"DENY"`) {
			t.Fatalf("expected authenticated DENY, got status=%d body=%s", rr.Code, rr.Body.String())
		}
	})

	t.Run("signed principal attribute is authoritative", func(t *testing.T) {
		body := `{"tenant_id":"tenant-a","subject":"user:alice","action":"READ","resource":"file:doc","context":{"principal.department":"engineering"}}`
		rr := call(body, makeToken("finance"))
		if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"decision":"ALLOW"`) {
			t.Fatalf("expected authenticated ALLOW, got status=%d body=%s", rr.Code, rr.Body.String())
		}
	})
}
