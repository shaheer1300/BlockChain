# test.ps1
# Runs the full quality gate: format, vet, and tests with no cache.
# Exits non-zero on the first failing step.

$ErrorActionPreference = "Stop"

Set-Location -Path (Join-Path $PSScriptRoot "..")

Write-Host "==> go fmt ./..." -ForegroundColor Cyan
go fmt ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> go vet ./..." -ForegroundColor Cyan
go vet ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host "==> go test ./... -count=1" -ForegroundColor Cyan
go test ./... -count=1
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

Write-Host ""
Write-Host "All checks passed." -ForegroundColor Green
