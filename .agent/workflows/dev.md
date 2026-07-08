# Standalone Policy Engine - Development Workflow

## 🚀 Setup & Run Local
1. **Prerequisites**:
   ```bash
   go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
   go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
   ```
2. **Compile Protobuf**:
   ```bash
   protoc --go_out=. --go-grpc_out=. proto/v1/policy.proto
   ```
3. **Run Server (Insecure Mode)**:
   ```bash
   go run cmd/pdp-server/main.go
   ```
4. **Run Server (OpenZiti Dark Mode)**:
   ```bash
   USE_ZITI=true ZITI_IDENTITY_PATH=docker/identities/pdp-dev.json ZITI_SERVICE_NAME=policy-decision-service go run cmd/pdp-server/main.go
   ```

## ⚡ Performance Testing & Benchmark
1. **gRPC CLI Verification**:
   ```bash
   grpcurl -plaintext -d '{"tenant_id": "tenant-a", "subject": "user:alice", "action": "READ", "resource": "balance"}' localhost:50051 policy.v1.PolicyDecisionPoint/CheckAccess
   ```
2. **Stress Test (ghz)**:
   ```bash
   ghz --insecure \
     --proto=proto/v1/policy.proto \
     --call=policy.v1.PolicyDecisionPoint.CheckAccess \
     -d '{"tenant_id": "tenant-a", "subject":"user:alice", "action":"READ", "resource":"balance"}' \
     -c 100 -n 100000 localhost:50051
   ```
   *Target*: Latency P99 < 1.0 ms. Success rate = 100%.
