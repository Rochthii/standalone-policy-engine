package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"

	"standalone-policy-engine/internal/storage"
	policyv1 "standalone-policy-engine/proto/v1"
)

const dockerComposeFile = "docker-compose.yml"

func TestE2E_DockerComposeFlow(t *testing.T) {
	requireDockerE2E(t)
	if output, err := dockerCompose("down", "--volumes", "--remove-orphans").CombinedOutput(); err != nil {
		t.Logf("pre-test cleanup: %s", output)
	}
	t.Cleanup(cleanupDockerE2E(t))

	if output, err := dockerCompose("up", "--build", "--detach").CombinedOutput(); err != nil {
		t.Fatalf("start Control Plane E2E stack: %v\n%s", err, output)
	}
	waitForServices(t)

	store, err := storage.NewStorage("postgres://postgres:postgres@localhost:5433/policy_engine?sslmode=disable")
	if err != nil {
		t.Fatalf("connect test PostgreSQL: %v", err)
	}
	defer store.Close()

	tenantID, err := store.CreateTenant(context.Background(), "tenant-e2e-test")
	if err != nil {
		t.Fatalf("create E2E tenant: %v", err)
	}
	adminAuth := generateJWT("test-secret-key-for-sprint-6-unit-test", tenantID, "user:admin")

	policyID := createPolicy(t, tenantID, adminAuth, `permit(principal == user:"alice", action == action:READ, resource == file:"report.pdf") when { true };`)
	publishPolicy(t, tenantID, policyID, adminAuth)

	conn, err := grpc.DialContext(context.Background(), "localhost:50061", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		t.Fatalf("dial PDP: %v", err)
	}
	defer conn.Close()
	client := policyv1.NewPolicyDecisionPointClient(conn)

	aliceRequest := &policyv1.CheckAccessRequest{TenantId: tenantID, Subject: "user:alice", Action: "READ", Resource: "file:report.pdf"}
	aliceContext := authorizationContext(generateJWT("test-secret-key-for-sprint-6-unit-test", tenantID, "user:alice"))
	waitForDecision(t, client, aliceContext, aliceRequest, policyv1.CheckAccessResponse_ALLOW)

	bobRequest := &policyv1.CheckAccessRequest{TenantId: tenantID, Subject: "user:bob", Action: "READ", Resource: "file:report.pdf"}
	bobContext := authorizationContext(generateJWT("test-secret-key-for-sprint-6-unit-test", tenantID, "user:bob"))
	denied, err := client.CheckAccess(bobContext, bobRequest)
	if err != nil || denied.Decision != policyv1.CheckAccessResponse_DENY {
		t.Fatalf("default deny before bob policy: response=%v err=%v", denied, err)
	}

	bobPolicyID := createPolicy(t, tenantID, adminAuth, `permit(principal == user:"bob", action == action:READ, resource == file:"report.pdf") when { true };`)
	publishPolicy(t, tenantID, bobPolicyID, adminAuth)
	waitForDecision(t, client, bobContext, bobRequest, policyv1.CheckAccessResponse_ALLOW)
}

func requireDockerE2E(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("docker"); err == nil {
		if err := exec.Command("docker", "info").Run(); err == nil {
			return
		}
	}
	if os.Getenv("REQUIRE_DOCKER_E2E") == "1" {
		t.Fatal("Docker CLI and daemon are required for this E2E gate")
	}
	t.Skip("Docker CLI and daemon are unavailable")
}

func cleanupDockerE2E(t *testing.T) func() {
	return func() {
		if t.Failed() {
			if output, err := dockerCompose("logs", "--no-color").CombinedOutput(); err == nil {
				t.Logf("Control Plane E2E logs:\n%s", output)
			}
		}
		if output, err := dockerCompose("down", "--volumes", "--remove-orphans").CombinedOutput(); err != nil {
			t.Logf("E2E cleanup failed: %v\n%s", err, output)
		}
	}
}

func dockerCompose(args ...string) *exec.Cmd {
	command := append([]string{"compose", "-f", dockerComposeFile}, args...)
	cmd := exec.Command("docker", command...)
	cmd.Dir = "."
	return cmd
}

func waitForServices(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(45 * time.Second)
	for time.Now().Before(deadline) {
		if endpointReady("http://localhost:8081/readyz") && endpointReady("http://localhost:8082/readyz") {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			conn, err := grpc.DialContext(ctx, "localhost:50061", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
			cancel()
			if err == nil {
				_ = conn.Close()
				return
			}
		}
		time.Sleep(250 * time.Millisecond)
	}
	t.Fatal("Control Plane, PDP and PostgreSQL did not become ready")
}

func endpointReady(endpoint string) bool {
	client := &http.Client{Timeout: time.Second}
	response, err := client.Get(endpoint)
	if err != nil {
		return false
	}
	defer response.Body.Close()
	return response.StatusCode == http.StatusOK
}

func createPolicy(t *testing.T, tenantID, authorization, policyText string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"effect": "permit", "policy_text": policyText})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies", tenantID), bytes.NewBuffer(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", authorization)
	request.Header.Set("Content-Type", "application/json")
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("create policy: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("create policy: HTTP %d: %s", response.StatusCode, payload)
	}
	var result struct {
		ID       string `json:"id"`
		PolicyID string `json:"policy_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		t.Fatalf("decode created policy: %v", err)
	}
	if result.ID != "" {
		return result.ID
	}
	if result.PolicyID == "" {
		t.Fatal("create policy returned no policy identifier")
	}
	return result.PolicyID
}

func publishPolicy(t *testing.T, tenantID, policyID, authorization string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies/%s/publish", tenantID, policyID), nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", authorization)
	response, err := (&http.Client{Timeout: 5 * time.Second}).Do(request)
	if err != nil {
		t.Fatalf("publish policy: %v", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("publish policy: HTTP %d: %s", response.StatusCode, payload)
	}
}

func authorizationContext(authorization string) context.Context {
	return metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", authorization))
}

func waitForDecision(t *testing.T, client policyv1.PolicyDecisionPointClient, ctx context.Context, request *policyv1.CheckAccessRequest, expected policyv1.CheckAccessResponse_Decision) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	var last string
	for time.Now().Before(deadline) {
		response, err := client.CheckAccess(ctx, request)
		if err == nil && response.Decision == expected {
			return
		}
		last = fmt.Sprintf("response=%v err=%v", response, err)
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("PDP decision did not converge to %s: %s", expected, last)
}

func generateJWT(secret, tenantID, subject string) string {
	issuer := os.Getenv("JWT_ISSUER")
	if issuer == "" {
		issuer = "standalone-policy-engine-test"
	}
	audience := os.Getenv("JWT_AUDIENCE")
	if audience == "" {
		audience = "standalone-policy-engine-pdp-test"
	}
	claims := jwt.MapClaims{
		"sub":         subject,
		"tenant_id":   tenantID,
		"permissions": []string{"policy:read", "policy:write", "policy:simulate", "policy:operate", "delegation:revoke"},
		"iss":         issuer,
		"aud":         audience,
		"exp":         time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return "Bearer " + tokenString
}
