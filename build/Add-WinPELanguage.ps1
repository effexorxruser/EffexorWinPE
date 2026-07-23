[CmdletBinding()]
param(
    [Parameter(Mandatory = $true)]
    [string]$MountDirectory,

    [string]$Locale = "ru-RU",

    [string]$AdkRoot = "",

    [ValidateSet("amd64")]
    [string]$Architecture = "amd64",

    [string[]]$OptionalComponents = @(),

    [switch]$DryRun,

    # Test hook: pre-parsed installed package identity strings (skips live DISM).
    [string[]]$MockInstalledPackages = @()
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

function Get-WinPEOptionalComponentName {
    param([string]$PackageIdentity)
    if ([string]::IsNullOrWhiteSpace($PackageIdentity)) {
        return $null
    }
    # Example identity:
    # Microsoft-Windows-WinPE-StorageWMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1
    $name = $null
    if ($PackageIdentity -match 'WinPE-([A-Za-z0-9\-]+)-Package') {
        $name = "WinPE-$($Matches[1])"
    }
    elseif ($PackageIdentity -match '^(WinPE-[A-Za-z0-9\-]+)$') {
        $name = $Matches[1]
    }
    if (-not $name) {
        return $null
    }
    # Foundation is part of the base image identity, not a localizable OC CAB.
    if ($name -eq "WinPE-Foundation") {
        return $null
    }
    return $name
}

function Get-InstalledWinPEOptionalComponentsFromDismOutput {
    param([string]$DismOutput)
    $names = New-Object System.Collections.Generic.List[string]
    foreach ($line in ($DismOutput -split "`r?`n")) {
        if ($line -match 'Package Identity\s*:\s*(.+)$') {
            $identity = $Matches[1].Trim()
            $name = Get-WinPEOptionalComponentName -PackageIdentity $identity
            if ($name -and -not $names.Contains($name)) {
                $names.Add($name) | Out-Null
            }
        }
    }
    return @($names)
}

function Get-InstalledWinPEOptionalComponents {
    param(
        [string]$MountDirectory,
        [string[]]$MockPackages
    )
    if ($MockPackages -and $MockPackages.Count -gt 0) {
        $names = New-Object System.Collections.Generic.List[string]
        foreach ($identity in $MockPackages) {
            $name = Get-WinPEOptionalComponentName -PackageIdentity $identity
            if ($name -and -not $names.Contains($name)) {
                $names.Add($name) | Out-Null
            }
        }
        return @($names)
    }
    $output = & dism.exe "/Image:$MountDirectory" /Get-Packages 2>&1 | Out-String
    if ($LASTEXITCODE -ne 0) {
        throw "DISM /Get-Packages failed with exit code $LASTEXITCODE.`n$output"
    }
    return Get-InstalledWinPEOptionalComponentsFromDismOutput -DismOutput $output
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

$InstalledComponents = Get-InstalledWinPEOptionalComponents -MountDirectory $MountDirectory -MockPackages $MockInstalledPackages
Write-Host "Installed WinPE optional components detected: $($InstalledComponents -join ', ')"

if ($OptionalComponents -and $OptionalComponents.Count -gt 0) {
    foreach ($Component in $OptionalComponents) {
        $baseName = $Component -replace "\.cab$", ""
        if ($InstalledComponents -notcontains $baseName) {
            throw "Optional component '$baseName' was requested, but DISM does not report it as installed in the mounted image."
        }
    }
    $ComponentsToLocalize = @($OptionalComponents | ForEach-Object { $_ -replace "\.cab$", "" })
}
else {
    $ComponentsToLocalize = @($InstalledComponents)
}

$PackagesToAdd = New-Object System.Collections.Generic.List[string]
$PackagesToAdd.Add($BaseLpPath) | Out-Null

foreach ($baseName in $ComponentsToLocalize) {
    $localized = "{0}_{1}.cab" -f $baseName, $LocaleFolder
    $localizedPath = Join-Path $LocaleRoot $localized
    $neutralPath = Join-Path $OptionalRoot ($baseName + ".cab")
    if (-not (Test-Path $neutralPath)) {
        Write-Host "Skipping localized package; neutral OC CAB missing: $neutralPath"
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

& dism.exe "/Image:$MountDirectory" /Set-AllIntl:$Locale
if ($LASTEXITCODE -ne 0) {
    throw "DISM /Set-AllIntl:$Locale failed with exit code $LASTEXITCODE."
}

& dism.exe "/Image:$MountDirectory" /Set-InputLocale:"0409:00000409;0419:00000419"
if ($LASTEXITCODE -ne 0) {
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
if ($LASTEXITCODE -ne 0) {
    Write-Warning "DISM /Set-LayeredDriver returned $LASTEXITCODE (ignored)."
}

Write-Host "WinPE language configuration applied for $Locale with English fallback keyboards."
