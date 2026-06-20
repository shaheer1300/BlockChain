# run-node1.ps1
# Starts node1 of the local 3-node devnet.
#
# Ports:
#   node1: 8001  <-- this node
#   node2: 8002
#   node3: 8003

$ErrorActionPreference = "Stop"

# Move to the project root (parent of scripts/).
Set-Location -Path (Join-Path $PSScriptRoot "..")

$env:NODE_ID                  = "node1"
$env:NETWORK_ID               = "localdev"
$env:HTTP_ADDR                = "127.0.0.1:8001"
$env:DATA_DIR                 = "./data/node1"
$env:PEERS                    = "http://127.0.0.1:8002,http://127.0.0.1:8003"
$env:LOG_LEVEL                = "info"
$env:POW_TARGET_PREFIX_ZEROES = "4"

Write-Host "Starting $($env:NODE_ID) on $($env:HTTP_ADDR)" -ForegroundColor Cyan
Write-Host "Peers: $($env:PEERS)" -ForegroundColor DarkGray
Write-Host "DataDir: $($env:DATA_DIR)" -ForegroundColor DarkGray
Write-Host ""

go run ./cmd/node
