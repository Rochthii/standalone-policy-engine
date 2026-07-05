# Script PowerShell để setup pectl trên Windows
# Chạy từ thư mục gốc project: .\scripts\setup-pectl.ps1

$ErrorActionPreference = "Stop"

Write-Host "==> Fetching pectl dependencies..." -ForegroundColor Cyan
& go get github.com/spf13/cobra@v1.8.0
& go get github.com/spf13/viper@v1.18.2
& go get gopkg.in/yaml.v3@v3.0.1
& go mod tidy

Write-Host "==> Building pectl binary..." -ForegroundColor Cyan

$VERSION  = "0.1.0"
$GIT_COMMIT = & git rev-parse --short HEAD 2>$null
if (-not $GIT_COMMIT) { $GIT_COMMIT = "none" }
$BUILD_TIME = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")

if (-not (Test-Path "bin")) { New-Item -ItemType Directory "bin" | Out-Null }

& go build `
  "-ldflags=-X standalone-policy-engine/internal/pectl/commands.Version=$VERSION -X standalone-policy-engine/internal/pectl/commands.GitCommit=$GIT_COMMIT -X standalone-policy-engine/internal/pectl/commands.BuildTime=$BUILD_TIME" `
  -o "bin\pectl.exe" `
  ".\cmd\pectl\"

Write-Host ""
Write-Host "==> Build complete: bin\pectl.exe" -ForegroundColor Green
Write-Host ""
& .\bin\pectl.exe version
