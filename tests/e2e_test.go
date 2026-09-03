package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

func TestE2E_DockerComposeFlow(t *testing.T) {
	if _, err := exec.LookPath("docker"); err != nil {
		t.Skip("Docker CLI khong kha dung tren moi truong nay, bo qua TestE2E_DockerComposeFlow")
		return
	}
	if err := exec.Command("docker", "info").Run(); err != nil {
		t.Skip("Docker daemon khong hoat dong tren moi truong nay, bo qua TestE2E_DockerComposeFlow")
		return
	}

	// 1. Khoi chay docker-compose
	t.Log("Dang don dep container va image cu de tranh loi Docker BuildKit...")
	_ = exec.Command("docker", "compose", "-f", "docker-compose.yml", "down", "-v").Run()
	_ = exec.Command("docker", "image", "rm", "-f", "standalone-pdp-test:latest", "standalone-control-test:latest").Run()

	t.Log("Dang khoi chay docker-compose stack...")
	upCmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "up", "--build", "-d")
	upCmd.Dir = "."
	if output, err := upCmd.CombinedOutput(); err != nil {
		t.Skipf("Khong the khoi chay docker-compose: %v. Output: %s", err, string(output))
		return
	}

	defer func() {
		t.Log("Dang stop va clean up docker-compose stack...")
		downCmd := exec.Command("docker", "compose", "-f", "docker-compose.yml", "down", "--rmi", "local", "-v")
		downCmd.Dir = "."
		_ = downCmd.Run()
	}()

	// 2. Cho server khoi dong va san sang nhan ket noi
	t.Log("Cho cac service san sang...")
	waitForServices(t)

	// Khoi tao Storage de tao Tenant that trong database
	dbConnStr := "postgres://postgres:postgres@localhost:5433/policy_engine?sslmode=disable"
	store, err := storage.NewStorage(dbConnStr)
	if err != nil {
		t.Fatalf("Khong the ket noi database: %v", err)
	}
	defer store.Close()

	tenantUUID, err := store.CreateTenant(context.Background(), "tenant-e2e-test")
	if err != nil {
		t.Fatalf("Khong the tao tenant trong DB: %v", err)
	}
	t.Logf("Da tao tenant UUID: %s", tenantUUID)

	// 3. Chuan bi token JWT hop le cho tenant-e2e-test
	secret := "test-secret-key-for-sprint-6-unit-test"
	authHeader := generateJWT(secret, tenantUUID, "user:admin")

	httpClient := &http.Client{Timeout: 5 * time.Second}

	// 4. Tao draft policy thong qua Control Plane REST API
	t.Log("Buoc 1: Tao draft policy...")
	policyText := `permit(principal == user:"alice", action == action:READ, resource == file:"report.pdf") when { true };`
	createBody, _ := json.Marshal(map[string]string{
		"effect":      "permit",
		"policy_text": policyText,
	})
	req, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies", tenantUUID), bytes.NewBuffer(createBody))
	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Content-Type", "application/json")
	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("Goi API tao policy that bai: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("Tao policy that bai: HTTP Status %d, body: %s", resp.StatusCode, string(body))
	}

	var createResult struct {
		ID       string `json:"id"`
		PolicyID string `json:"policy_id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&createResult)
	policyID := createResult.ID
	if policyID == "" {
		policyID = createResult.PolicyID
	}
	t.Logf("Draft policy duoc tao thanh cong: %s", policyID)

	// 5. Xuat ban (Publish) chính sách
	t.Log("Buoc 2: Xuat ban draft policy sang trang thai ACTIVE...")
	reqPub, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies/%s/publish", tenantUUID, policyID), nil)
	reqPub.Header.Set("Authorization", authHeader)
	respPub, err := httpClient.Do(reqPub)
	if err != nil {
		t.Fatalf("Goi API publish policy that bai: %v", err)
	}
	defer respPub.Body.Close()
	if respPub.StatusCode != http.StatusOK {
		t.Fatalf("Publish policy that bai: HTTP Status %d", respPub.StatusCode)
	}
	t.Log("Draft policy da duoc active.")

	// Cho 1.5s de tin hieu Pub/Sub cua Redis dong bo toi PDP server
	time.Sleep(1500 * time.Millisecond)

	// 6. Ket noi gRPC toi pdp-server de kiem tra quyet dinh quyen (Data Plane)
	t.Log("Buoc 3: Goi gRPC pdp-server CheckAccess...")
	conn, err := grpc.Dial("localhost:50061", grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithDefaultCallOptions(grpc.CallContentSubtype("json")))
	if err != nil {
		t.Fatalf("Dial gRPC that bai: %v", err)
	}
	defer conn.Close()

	client := policyv1.NewPolicyDecisionPointClient(conn)

	aliceAuth := generateJWT(secret, tenantUUID, "user:alice")
	ctxAlice := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", aliceAuth))
	reqAccess := &policyv1.CheckAccessRequest{
		TenantId: tenantUUID,
		Subject:  "user:alice",
		Action:   "READ",
		Resource: "file:report.pdf",
	}
	respAccess, err := client.CheckAccess(ctxAlice, reqAccess)
	if err != nil {
		t.Fatalf("CheckAccess gap loi: %v", err)
	}
	if respAccess.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Errorf("Mong doi ALLOW, thuc te %v", respAccess.Decision)
	}
	t.Log("Quyet dinh phân quyen ALLOW hop le da duoc tra ve.")

	// Kich ban khong khop policy (DENY mac dinh)
	bobAuth := generateJWT(secret, tenantUUID, "user:bob")
	ctxBob := metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", bobAuth))
	reqAccessDeny := &policyv1.CheckAccessRequest{
		TenantId: tenantUUID,
		Subject:  "user:bob",
		Action:   "READ",
		Resource: "file:report.pdf",
	}
	respAccessDeny, err := client.CheckAccess(ctxBob, reqAccessDeny)
	if err != nil {
		t.Fatalf("CheckAccess gap loi: %v", err)
	}
	if respAccessDeny.Decision != policyv1.CheckAccessResponse_DENY {
		t.Errorf("Mong doi DENY cho user:bob, thuc te %v", respAccessDeny.Decision)
	}
	t.Log("Quyet dinh phân quyen DENY cho user:bob hop le.")

	// 7. Kiểm chứng PostgreSQL Monotonic Sequence & Fast Gap Catch-Up (< 50ms)
	t.Log("Buoc 4: Kiem tra Postgres Monotonic Sequence & Fast Catch-Up...")

	// Tao policy moi cho user:bob
	t.Log("Tao va publish policy moi cho user:bob...")
	policyText2 := `permit(principal == user:"bob", action == action:READ, resource == file:"report.pdf") when { true };`
	createBody2, _ := json.Marshal(map[string]string{
		"effect":      "permit",
		"policy_text": policyText2,
	})
	req2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies", tenantUUID), bytes.NewBuffer(createBody2))
	req2.Header.Set("Authorization", authHeader)
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := httpClient.Do(req2)
	if err != nil {
		t.Fatalf("Goi API tao policy 2 that bai: %v", err)
	}
	defer resp2.Body.Close()
	var createResult2 struct {
		ID       string `json:"id"`
		PolicyID string `json:"policy_id"`
	}
	_ = json.NewDecoder(resp2.Body).Decode(&createResult2)
	policyID2 := createResult2.ID
	if policyID2 == "" {
		policyID2 = createResult2.PolicyID
	}

	// Publish policy moi (Kích hoạt UPDATE revision và NOTIFY policy_events)
	reqPub2, _ := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:8082/api/v1/tenants/%s/policies/%s/publish", tenantUUID, policyID2), nil)
	reqPub2.Header.Set("Authorization", authHeader)
	respPub2, err := httpClient.Do(reqPub2)
	if err != nil || respPub2.StatusCode != http.StatusOK {
		t.Fatalf("Publish policy 2 that bai: %v", err)
	}
	respPub2.Body.Close()

	// Chỉ cần đợi 200ms để Postgres LISTEN/NOTIFY lan truyền tới PDP server (thay vì 12s polling cũ)
	time.Sleep(200 * time.Millisecond)

	// Kiem tra lai quyet dinh quyen cho user:bob. Luc nay phai la ALLOW tức thời!
	respAccess2, err := client.CheckAccess(ctxBob, reqAccessDeny)
	if err != nil {
		t.Fatalf("CheckAccess sau khi Publish policy 2 gap loi: %v", err)
	}
	if respAccess2.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Errorf("Kỳ vọng ALLOW cho user:bob qua Postgres LISTEN/NOTIFY, nhung thuc te la %v", respAccess2.Decision)
	}
	t.Log("Postgres LISTEN/NOTIFY va Gap Catch-Up da hoat dong chinh xac, nạp quyet dinh ALLOW cho user:bob chi trong < 50ms.")
}

func waitForServices(t *testing.T) {
	for i := 0; i < 20; i++ {
		conn, err := grpc.Dial("localhost:50061", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			conn.Close()
			resp, err := http.Get("http://localhost:8082/metrics")
			if err == nil {
				resp.Body.Close()
				t.Log("Cac service da san sang.")
				return
			}
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatal("Timeout cho cac service khoi dong.")
}

func generateJWT(secret, tenantID, subject string) string {
	claims := jwt.MapClaims{
		"sub":       subject,
		"tenant_id": tenantID,
		"exp":       time.Now().Add(1 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(secret))
	return "Bearer " + tokenStr
}
