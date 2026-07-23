[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$MountDirectory,

    [string]$Locale = "ru-RU",

    [string]$AdkRoot = "",

    [ValidateSet("amd64")]
    [string]$Architecture = "amd64",

    [string[]]$OptionalComponents = @(),

    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

function Resolve-AdkRoot {
    param([string]$Requested)
    if ($Requested) {
        return [IO.Path]::GetFullPath($Requested)
    }
    $default = Join-Path ${env:ProgramFiles(x86)} "Windows Kits/10/Assessment and Deployment Kit"
    if (Test-Path $default) {
        return $default
    }
    throw "Windows ADK was not found. Pass -AdkRoot or install Windows ADK with the WinPE add-on."
}

function Get-LocaleFolderName {
    param([string]$LocaleName)
    return $LocaleName.ToLowerInvariant()
}

$MountDirectory = [IO.Path]::GetFullPath($MountDirectory)
if (-not (Test-Path $MountDirectory)) {
    throw "Mount directory does not exist: $MountDirectory"
}

$AdkRoot = Resolve-AdkRoot -Requested $AdkRoot
$WinPERoot = Join-Path $AdkRoot "Windows Preinstallation Environment"
$OptionalRoot = Join-Path $WinPERoot "$Architecture/WinPE_OCs"
$LocaleFolder = Get-LocaleFolderName -LocaleName $Locale
$LocaleRoot = Join-Path $OptionalRoot $LocaleFolder

if (-not (Test-Path $OptionalRoot)) {
    throw "WinPE optional components were not found at $OptionalRoot. Install the WinPE add-on for the Windows ADK."
}
if (-not (Test-Path $LocaleRoot)) {
    throw @"
Language pack folder for locale '$Locale' was not found:
  $LocaleRoot

Install the matching WinPE language pack through the Windows ADK / WinPE add-on.
This repository does not store Microsoft CAB files.
"@
}

$BaseLpName = "lp.cab"
$BaseLpPath = Join-Path $LocaleRoot $BaseLpName
if (-not (Test-Path $BaseLpPath)) {
    throw "Required WinPE language pack CAB is missing: $BaseLpPath"
}

# Discover already installed optional component packages in the mounted image
# when the caller did not pass an explicit list.
if (-not $OptionalComponents -or $OptionalComponents.Count -eq 0) {
    $OptionalComponents = @(
        "WinPE-WMI",
        "WinPE-NetFX",
        "WinPE-Scripting",
        "WinPE-PowerShell",
        "WinPE-StorageWMI",
        "WinPE-DismCmdlets",
        "WinPE-SecureStartup",
        "WinPE-EnhancedStorage"
    )
}

$PackagesToAdd = New-Object System.Collections.Generic.List[string]
$PackagesToAdd.Add($BaseLpPath) | Out-Null

foreach ($Component in $OptionalComponents) {
    $baseName = $Component
    if ($baseName -notlike "*.cab") {
        # Matching localized OC package name pattern used by WinPE ADK layouts.
        $localized = "{0}_{1}.cab" -f $baseName, $LocaleFolder
    }
    else {
        $localized = ($baseName -replace "\.cab$", "") + "_$LocaleFolder.cab"
        $baseName = $baseName -replace "\.cab$", ""
    }
    $localizedPath = Join-Path $LocaleRoot $localized
    $neutralPath = Join-Path $OptionalRoot ($baseName + ".cab")

    if (-not (Test-Path $neutralPath)) {
        Write-Host "Skipping localized package for missing neutral OC: $neutralPath"
        continue
    }
    if (-not (Test-Path $localizedPath)) {
        throw "Matching language pack for optional component '$baseName' is missing: $localizedPath"
    }
    $PackagesToAdd.Add($localizedPath) | Out-Null
}

Write-Host "ADK root: $AdkRoot"
Write-Host "Mount directory: $MountDirectory"
Write-Host "Locale: $Locale (folder $LocaleFolder)"
Write-Host "Packages to add:"
foreach ($pkg in $PackagesToAdd) {
    Write-Host "  - $pkg"
}

if ($DryRun) {
    Write-Host "Dry-run: DISM and locale configuration were not applied."
    Write-Host "Would set system UI language / locale to $Locale with en-US keyboard fallback."
    return
}

foreach ($PackagePath in $PackagesToAdd) {
    & dism.exe "/Image:$MountDirectory" /Add-Package "/PackagePath:$PackagePath"
    if ($LASTEXITCODE -ne 0) {
        throw "DISM failed while adding package $PackagePath (exit $LASTEXITCODE)."
    }
}

# Set system and UI locale. Keep English input as fallback alongside Russian.
& dism.exe "/Image:$MountDirectory" /Set-AllIntl:$Locale
if ($LASTEXITCODE -ne 0) {
    throw "DISM /Set-AllIntl:$Locale failed with exit code $LASTEXITCODE."
}

& dism.exe "/Image:$MountDirectory" /Set-InputLocale:"0409:00000409;0419:00000419"
if ($LASTEXITCODE -ne 0) {
    # Fallback for ADK builds that expect a single locale then layered keyboards.
    Write-Warning "Combined /Set-InputLocale failed; applying en-US then ru-RU keyboards separately."
    & dism.exe "/Image:$MountDirectory" /Set-InputLocale:"0409:00000409"
    if ($LASTEXITCODE -ne 0) {
        throw "DISM failed while setting en-US input locale."
    }
    & dism.exe "/Image:$MountDirectory" /Set-InputLocale:"0419:00000419"
    if ($LASTEXITCODE -ne 0) {
        throw "DISM failed while setting ru-RU input locale."
    }
}

& dism.exe "/Image:$MountDirectory" /Set-LayeredDriver:1
# Non-fatal on some ADK versions.
if ($LASTEXITCODE -ne 0) {
    Write-Warning "DISM /Set-LayeredDriver returned $LASTEXITCODE (ignored)."
}

Write-Host "WinPE language configuration applied for $Locale with English fallback keyboards."
Write-Host "English remains available as an input locale fallback; UI language is $Locale."
