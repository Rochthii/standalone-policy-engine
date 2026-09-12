.PHONY: test bench lint build tidy generate-proto proto-check build-pectl install-pectl test-pectl test-odoo-e2e run-pdp run-control pdp control-plane migrate docker

test:
	go test -v ./...

bench:
	go test -v -bench=. -run=^$$ ./...

lint:
	golangci-lint run

build:
	go build ./...

tidy:
	go mod tidy

generate-proto:
	go run github.com/bufbuild/buf/cmd/buf@v1.72.0 generate

proto-check:
	go run github.com/bufbuild/buf/cmd/buf@v1.72.0 lint
	go run github.com/bufbuild/buf/cmd/buf@v1.72.0 format --diff --exit-code
	go run github.com/bufbuild/buf/cmd/buf@v1.72.0 generate
	git diff --exit-code -- proto/v1/policy.pb.go proto/v1/policy_grpc.pb.go clients/python

build-pectl:
	go build -ldflags="-X standalone-policy-engine/internal/pectl/commands.Version=0.1.0 \
	  -X standalone-policy-engine/internal/pectl/commands.GitCommit=$$(git rev-parse --short HEAD 2>/dev/null || echo none) \
	  -X standalone-policy-engine/internal/pectl/commands.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
	  -o bin/pectl ./cmd/pectl/

install-pectl: build-pectl
	cp bin/pectl $(GOPATH)/bin/pectl || cp bin/pectl ~/go/bin/pectl

test-pectl:
	go test -v -cover ./internal/pectl/...

test-odoo-e2e:
	docker compose -f docker-compose.testbed.yml --profile e2e up --build --abort-on-container-exit --exit-code-from testbed-odoo-e2e testbed-odoo-e2e

run-pdp:
	go run ./cmd/pdp-server/main.go

pdp: run-pdp

run-control:
	go run ./cmd/control-plane/main.go

control-plane: run-control

migrate:
	migrate -path db/migrations -database "$$DATABASE_URL" up

docker:
	docker compose -f tests/docker-compose.yml up -d postgres redis
