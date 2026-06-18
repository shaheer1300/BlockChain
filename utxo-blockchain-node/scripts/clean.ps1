# clean.ps1
# Removes build artifacts and devnet data.
#
# Deletes:
#   bin/                 build output
#   data/                bbolt databases for all devnet nodes
#   node.exe             stray binary at repo root (legacy build target)

$ErrorActionPreference = "Stop"

Set-Location -Path (Join-Path $PSScriptRoot "..")

$targets = @("bin", "data", "node.exe")

foreach ($t in $targets) {
    if (Test-Path $t) {
        Write-Host "Removing $t" -ForegroundColor Yellow
        Remove-Item -Path $t -Recurse -Force
    } else {
        Write-Host "Skip $t (not present)" -ForegroundColor DarkGray
    }
}

Write-Host "Clean complete." -ForegroundColor Green
