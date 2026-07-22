[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $RepoRoot
try {
    Get-Content "contracts/diagnostic-report.schema.json" -Raw | ConvertFrom-Json | Out-Null
    Get-Content "contracts/diagnosis.schema.json" -Raw | ConvertFrom-Json | Out-Null
    Get-Content "manifests/tools.json" -Raw | ConvertFrom-Json | Out-Null
    Get-Content "manifests/drivers.json" -Raw | ConvertFrom-Json | Out-Null
    Get-Content "manifests/image-payload.json" -Raw | ConvertFrom-Json | Out-Null

    & (Join-Path $PSScriptRoot "Test-PayloadAllowlist.ps1")

    if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
        throw "Go was not found. Install Go 1.24 or newer."
    }
    & go test ./...
    if ($LASTEXITCODE -ne 0) {
        throw "Go tests failed with exit code $LASTEXITCODE."
    }

    Write-Host "Repository checks passed."
}
finally {
    Pop-Location
}
