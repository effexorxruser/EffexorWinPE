[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$TestRoot = Join-Path ([IO.Path]::GetTempPath()) ("EffexorWinPE-payload-test-" + [Guid]::NewGuid())
$SourceRoot = Join-Path $TestRoot "source"
$DestinationRoot = Join-Path $TestRoot "destination"
$ManifestPath = Join-Path $TestRoot "manifest.json"
$TraversalManifestPath = Join-Path $TestRoot "traversal-manifest.json"

try {
    New-Item -ItemType Directory -Force -Path (Join-Path $SourceRoot "payload/bin") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $SourceRoot "payload/reports") | Out-Null
    New-Item -ItemType Directory -Force -Path (Join-Path $SourceRoot "payload/secrets") | Out-Null
    Set-Content -LiteralPath (Join-Path $SourceRoot "payload/bin/tool.exe") -Value "safe"
    Set-Content -LiteralPath (Join-Path $SourceRoot "payload/reports/client.json") -Value "client data"
    Set-Content -LiteralPath (Join-Path $SourceRoot "payload/secrets/device-token.txt") -Value "secret"

    @{
        schema_version = 1
        files = @(
            @{
                source = "payload/bin/tool.exe"
                destination = "bin/tool.exe"
            }
        )
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $ManifestPath

    & (Join-Path $PSScriptRoot "Copy-ImagePayload.ps1") `
        -SourceRoot $SourceRoot `
        -DestinationRoot $DestinationRoot `
        -ManifestPath $ManifestPath

    if (-not (Test-Path -LiteralPath (Join-Path $DestinationRoot "bin/tool.exe") -PathType Leaf)) {
        throw "Allowlisted payload file was not copied."
    }
    if (Test-Path -LiteralPath (Join-Path $DestinationRoot "reports/client.json")) {
        throw "Unlisted client report was copied into the image payload."
    }
    if (Test-Path -LiteralPath (Join-Path $DestinationRoot "secrets/device-token.txt")) {
        throw "Unlisted secret was copied into the image payload."
    }

    @{
        schema_version = 1
        files = @(
            @{
                source = "payload/bin/tool.exe"
                destination = "../escaped.exe"
            }
        )
    } | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath $TraversalManifestPath

    $TraversalWasRejected = $false
    try {
        & (Join-Path $PSScriptRoot "Copy-ImagePayload.ps1") `
            -SourceRoot $SourceRoot `
            -DestinationRoot $DestinationRoot `
            -ManifestPath $TraversalManifestPath
    }
    catch {
        $TraversalWasRejected = $true
    }
    if (-not $TraversalWasRejected) {
        throw "Payload destination path traversal was not rejected."
    }

    Write-Host "Payload allowlist checks passed."
}
finally {
    if (Test-Path -LiteralPath $TestRoot) {
        Remove-Item -LiteralPath $TestRoot -Recurse -Force
    }
}
