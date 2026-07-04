.PHONY: test bench lint build run-pdp run-control pdp control-plane migrate docker

test:
	go test -v ./...

bench:
	go test -v -bench=. -run=^$$ ./...

lint:
	golangci-lint run

build:
	go build ./...

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
