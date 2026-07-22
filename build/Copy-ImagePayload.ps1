[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$SourceRoot,
    [Parameter(Mandatory = $true)]
    [string]$DestinationRoot,
    [Parameter(Mandatory = $true)]
    [string]$ManifestPath
)

$ErrorActionPreference = "Stop"

function Resolve-ContainedPath {
    param(
        [string]$Root,
        [string]$RelativePath,
        [string]$Description
    )

    if ([string]::IsNullOrWhiteSpace($RelativePath)) {
        throw "$Description must not be empty."
    }
    if ([IO.Path]::IsPathRooted($RelativePath)) {
        throw "$Description must be relative: $RelativePath"
    }

    $RootPath = [IO.Path]::GetFullPath($Root)
    $CandidatePath = [IO.Path]::GetFullPath((Join-Path $RootPath $RelativePath))
    $RootPrefix = $RootPath.TrimEnd(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    ) + [IO.Path]::DirectorySeparatorChar

    if (-not $CandidatePath.StartsWith($RootPrefix, [StringComparison]::OrdinalIgnoreCase)) {
        throw "$Description escapes its allowed root: $RelativePath"
    }

    return $CandidatePath
}

function Assert-NoReparsePoints {
    param(
        [string]$Root,
        [string]$Path
    )

    $RootPath = [IO.Path]::GetFullPath($Root)
    $RelativePath = $Path.Substring($RootPath.Length).TrimStart(
        [IO.Path]::DirectorySeparatorChar,
        [IO.Path]::AltDirectorySeparatorChar
    )
    $CurrentPath = $RootPath
    foreach ($Segment in ($RelativePath -split '[\\/]')) {
        if ([string]::IsNullOrWhiteSpace($Segment)) {
            continue
        }
        $CurrentPath = Join-Path $CurrentPath $Segment
        $Item = Get-Item -LiteralPath $CurrentPath -Force
        if (($Item.Attributes -band [IO.FileAttributes]::ReparsePoint) -ne 0) {
            throw "Allowlisted payload source contains a reparse point: $CurrentPath"
        }
    }
}

if (-not (Test-Path -LiteralPath $ManifestPath -PathType Leaf)) {
    throw "Payload manifest was not found: $ManifestPath"
}

$Manifest = Get-Content -LiteralPath $ManifestPath -Raw | ConvertFrom-Json
if ($Manifest.schema_version -ne 1) {
    throw "Unsupported payload manifest schema version: $($Manifest.schema_version)"
}
if ($null -eq $Manifest.files -or $Manifest.files.Count -eq 0) {
    throw "Payload manifest does not contain any files."
}

$SourceRoot = [IO.Path]::GetFullPath($SourceRoot)
$DestinationRoot = [IO.Path]::GetFullPath($DestinationRoot)
New-Item -ItemType Directory -Force -Path $DestinationRoot | Out-Null

$Destinations = @{}
foreach ($File in $Manifest.files) {
    $SourcePath = Resolve-ContainedPath $SourceRoot $File.source "Payload source"
    $DestinationPath = Resolve-ContainedPath $DestinationRoot $File.destination "Payload destination"
    $DestinationKey = $DestinationPath.ToUpperInvariant()

    if ($Destinations.ContainsKey($DestinationKey)) {
        throw "Payload manifest contains a duplicate destination: $($File.destination)"
    }
    $Destinations[$DestinationKey] = $true

    if (-not (Test-Path -LiteralPath $SourcePath -PathType Leaf)) {
        throw "Allowlisted payload source was not found: $SourcePath"
    }
    Assert-NoReparsePoints $SourceRoot $SourcePath

    $DestinationParent = Split-Path -Parent $DestinationPath
    New-Item -ItemType Directory -Force -Path $DestinationParent | Out-Null
    Copy-Item -LiteralPath $SourcePath -Destination $DestinationPath -Force
    Write-Host "Included payload file: $($File.destination)"
}
