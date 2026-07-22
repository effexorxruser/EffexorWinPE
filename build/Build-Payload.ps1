[CmdletBinding()]
param(
    [string]$Version = "0.1.0-dev"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
$PayloadBin = Join-Path $RepoRoot "payload/ANP/bin"

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
    $Targets = @(
        @{ Package = "./cmd/anp-collector"; Output = "anp-collector.exe" },
        @{ Package = "./cmd/anp-agent"; Output = "anp-agent.exe" }
    )
    foreach ($Target in $Targets) {
        $OutputPath = Join-Path $PayloadBin $Target.Output
        & go build -trimpath -ldflags "-s -w -X main.version=$Version" -o $OutputPath $Target.Package
        if ($LASTEXITCODE -ne 0) {
            throw "Go build failed for $($Target.Package) with exit code $LASTEXITCODE."
        }
        Write-Host "Payload executable ready: $OutputPath"
    }
}
finally {
    $env:GOOS = $PreviousGoOS
    $env:GOARCH = $PreviousGoArch
    $env:CGO_ENABLED = $PreviousCgo
    Pop-Location
}

Write-Host "Payload ready: $PayloadBin"
