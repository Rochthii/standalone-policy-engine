.PHONY: test bench lint build tidy build-pectl install-pectl test-pectl run-pdp run-control pdp control-plane migrate docker

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

build-pectl:
	go build -ldflags="-X standalone-policy-engine/internal/pectl/commands.Version=0.1.0 \
	  -X standalone-policy-engine/internal/pectl/commands.GitCommit=$$(git rev-parse --short HEAD 2>/dev/null || echo none) \
	  -X standalone-policy-engine/internal/pectl/commands.BuildTime=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
	  -o bin/pectl ./cmd/pectl/

install-pectl: build-pectl
	cp bin/pectl $(GOPATH)/bin/pectl || cp bin/pectl ~/go/bin/pectl

test-pectl:
	go test -v -cover ./internal/pectl/...

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
