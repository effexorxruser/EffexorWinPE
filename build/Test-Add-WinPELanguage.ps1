# Dry-run unit checks for Add-WinPELanguage.ps1 (no live ADK/DISM required).
$ErrorActionPreference = "Stop"
$Script = Join-Path $PSScriptRoot "Add-WinPELanguage.ps1"

function Get-WinPEOptionalComponentName {
    param([string]$PackageIdentity)
    if ([string]::IsNullOrWhiteSpace($PackageIdentity)) {
        return $null
    }
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

$sample = @"
Deployment Image Servicing and Management tool
Version: 10.0.22621.1

Image Version: 10.0.22621.1

Packages listing:

Package Identity : Microsoft-Windows-WinPE-Foundation-Package~31bf3856ad364e35~amd64~~10.0.22621.1
State : Installed

Package Identity : Microsoft-Windows-WinPE-WMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1
State : Installed

Package Identity : Microsoft-Windows-WinPE-StorageWMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1
State : Installed

Package Identity : Microsoft-Windows-Client-LanguagePack-Package~31bf3856ad364e35~amd64~ru-RU~10.0.22621.1
State : Installed

The operation completed successfully.
"@

$parsed = Get-InstalledWinPEOptionalComponentsFromDismOutput -DismOutput $sample
if ($parsed -contains "WinPE-Foundation") { throw "Foundation must not be treated as a localizable OC" }
if ($parsed -notcontains "WinPE-WMI") { throw "expected WinPE-WMI" }
if ($parsed -notcontains "WinPE-StorageWMI") { throw "expected WinPE-StorageWMI" }
if ($parsed.Count -ne 2) { throw "expected only WMI/StorageWMI, got: $($parsed -join ',')" }

# Requested component must be present in installed set.
$installed = @("WinPE-WMI", "WinPE-StorageWMI")
$requested = @("WinPE-PowerShell")
$validatedMissing = $false
foreach ($Component in $requested) {
    if ($installed -notcontains $Component) {
        $validatedMissing = $true
    }
}
if (-not $validatedMissing) {
    throw "expected validation failure for missing requested component"
}

# Full-script dry-run with mocked DISM package list and a fake ADK OC tree.
$tempRoot = Join-Path ([IO.Path]::GetTempPath()) ("effexor-winpe-lang-test-" + [guid]::NewGuid().ToString("N"))
$mountDir = Join-Path $tempRoot "mount"
$adkRoot = Join-Path $tempRoot "adk"
$ocRoot = Join-Path $adkRoot "Windows Preinstallation Environment/amd64/WinPE_OCs"
$localeRoot = Join-Path $ocRoot "ru-ru"
New-Item -ItemType Directory -Force -Path $mountDir, $localeRoot | Out-Null
Set-Content -Path (Join-Path $localeRoot "lp.cab") -Value "fake-lp"
Set-Content -Path (Join-Path $ocRoot "WinPE-WMI.cab") -Value "fake-wmi"
Set-Content -Path (Join-Path $localeRoot "WinPE-WMI_ru-ru.cab") -Value "fake-wmi-ru"
Set-Content -Path (Join-Path $ocRoot "WinPE-StorageWMI.cab") -Value "fake-storage"
Set-Content -Path (Join-Path $localeRoot "WinPE-StorageWMI_ru-ru.cab") -Value "fake-storage-ru"

try {
    & $Script `
        -MountDirectory $mountDir `
        -AdkRoot $adkRoot `
        -Locale "ru-RU" `
        -DryRun `
        -MockInstalledPackages @(
            "Microsoft-Windows-WinPE-Foundation-Package~31bf3856ad364e35~amd64~~10.0.22621.1",
            "Microsoft-Windows-WinPE-WMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1",
            "Microsoft-Windows-WinPE-StorageWMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1"
        )

    $failed = $false
    try {
        & $Script `
            -MountDirectory $mountDir `
            -AdkRoot $adkRoot `
            -Locale "ru-RU" `
            -DryRun `
            -OptionalComponents @("WinPE-PowerShell") `
            -MockInstalledPackages @(
                "Microsoft-Windows-WinPE-WMI-Package~31bf3856ad364e35~amd64~~10.0.22621.1"
            )
    }
    catch {
        $failed = $true
    }
    if (-not $failed) {
        throw "expected DryRun to fail when -OptionalComponents is not installed"
    }
}
finally {
    Remove-Item -Recurse -Force $tempRoot -ErrorAction SilentlyContinue
}

Write-Host "Add-WinPELanguage helper dry-run tests passed."
