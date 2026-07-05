#!/usr/bin/env bash
# Script to setup pectl: fetch dependencies and build binary
# Run this from the project root: bash scripts/setup-pectl.sh

set -e

echo "==> Fetching pectl dependencies..."
go get github.com/spf13/cobra@v1.8.0
go get github.com/spf13/viper@v1.18.2
go get gopkg.in/yaml.v3@v3.0.1
go mod tidy

echo "==> Building pectl binary..."
VERSION="0.1.0"
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

mkdir -p bin
go build \
  -ldflags="-X standalone-policy-engine/internal/pectl/commands.Version=${VERSION} \
             -X standalone-policy-engine/internal/pectl/commands.GitCommit=${GIT_COMMIT} \
             -X standalone-policy-engine/internal/pectl/commands.BuildTime=${BUILD_TIME}" \
  -o bin/pectl \
  ./cmd/pectl/

echo ""
echo "==> Build complete: bin/pectl"
echo ""
./bin/pectl version
