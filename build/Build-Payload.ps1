[CmdletBinding()]
param(
    [string]$Version = "0.1.0-dev"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$PayloadBin = Join-Path $RepoRoot "payload/ANP/bin"
$Collector = Join-Path $PayloadBin "anp-collector.exe"

if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw "Go was not found. Install Go 1.24 or newer and reopen PowerShell."
}

New-Item -ItemType Directory -Force -Path $PayloadBin | Out-Null

$PreviousGoOS = $env:GOOS
$PreviousGoArch = $env:GOARCH
$PreviousCgo = $env:CGO_ENABLED
Push-Location $RepoRoot
try {
    $env:GOOS = "windows"
    $env:GOARCH = "amd64"
    $env:CGO_ENABLED = "0"
    & go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $Collector ./cmd/anp-collector
    if ($LASTEXITCODE -ne 0) {
        throw "Go build failed with exit code $LASTEXITCODE."
    }
}
finally {
    $env:GOOS = $PreviousGoOS
    $env:GOARCH = $PreviousGoArch
    $env:CGO_ENABLED = $PreviousCgo
    Pop-Location
}

Write-Host "Payload ready: $Collector"
