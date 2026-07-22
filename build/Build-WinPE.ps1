[CmdletBinding()]
param(
    [ValidateSet("amd64")]
    [string]$Architecture = "amd64",
    [string]$Language = "en-us",
    [string]$OutputDirectory = "",
    [switch]$IncludeLocalDrivers,
    [switch]$BootEx
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path -Parent $PSScriptRoot
if (-not $OutputDirectory) {
    $OutputDirectory = Join-Path $RepoRoot "out"
}
$OutputDirectory = [IO.Path]::GetFullPath($OutputDirectory)
$WorkingDirectory = Join-Path $OutputDirectory "winpe-$Architecture"
$MountDirectory = Join-Path $WorkingDirectory "mount"
$IsoPath = Join-Path $OutputDirectory "EffexorWinPE-$Architecture.iso"

$AdkRoot = Join-Path ${env:ProgramFiles(x86)} "Windows Kits/10/Assessment and Deployment Kit"
$WinPERoot = Join-Path $AdkRoot "Windows Preinstallation Environment"
$CopyPE = Join-Path $WinPERoot "copype.cmd"
$MakeWinPEMedia = Join-Path $WinPERoot "MakeWinPEMedia.cmd"
$OptionalComponents = Join-Path $WinPERoot "$Architecture/WinPE_OCs"

if (-not (Test-Path $CopyPE)) {
    throw "Windows ADK Deployment Tools were not found at $CopyPE."
}
if (-not (Test-Path $MakeWinPEMedia)) {
    throw "Windows PE add-on was not found at $MakeWinPEMedia."
}

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "Build-WinPE.ps1 must run from an elevated PowerShell session."
}

& (Join-Path $PSScriptRoot "Build-Payload.ps1")

New-Item -ItemType Directory -Force -Path $OutputDirectory | Out-Null
if (Test-Path $WorkingDirectory) {
    Remove-Item -Recurse -Force $WorkingDirectory
}

& $CopyPE $Architecture $WorkingDirectory
if ($LASTEXITCODE -ne 0) {
    throw "copype failed with exit code $LASTEXITCODE."
}

$BootWim = Join-Path $WorkingDirectory "media/sources/boot.wim"
$Mounted = $false
try {
    & dism.exe /Mount-Image "/ImageFile:$BootWim" /Index:1 "/MountDir:$MountDirectory"
    if ($LASTEXITCODE -ne 0) {
        throw "DISM failed to mount boot.wim."
    }
    $Mounted = $true

    $Packages = @(
        "WinPE-WMI.cab",
        "WinPE-NetFX.cab",
        "WinPE-Scripting.cab",
        "WinPE-PowerShell.cab",
        "WinPE-StorageWMI.cab",
        "WinPE-DismCmdlets.cab",
        "WinPE-SecureStartup.cab",
        "WinPE-EnhancedStorage.cab"
    )
    foreach ($Package in $Packages) {
        $PackagePath = Join-Path $OptionalComponents $Package
        if (-not (Test-Path $PackagePath)) {
            throw "Required WinPE optional component is missing: $PackagePath"
        }
        & dism.exe "/Image:$MountDirectory" /Add-Package "/PackagePath:$PackagePath"
        if ($LASTEXITCODE -ne 0) {
            throw "DISM failed while adding $Package."
        }

        $LocalizedName = $Package -replace "\.cab$", "_$Language.cab"
        $LocalizedPath = Join-Path $OptionalComponents "$Language/$LocalizedName"
        if (Test-Path $LocalizedPath) {
            & dism.exe "/Image:$MountDirectory" /Add-Package "/PackagePath:$LocalizedPath"
            if ($LASTEXITCODE -ne 0) {
                throw "DISM failed while adding $LocalizedName."
            }
        }
    }

    $PayloadTarget = Join-Path $MountDirectory "EffexorWinPE"
    & (Join-Path $PSScriptRoot "Copy-ImagePayload.ps1") `
        -SourceRoot $RepoRoot `
        -DestinationRoot $PayloadTarget `
        -ManifestPath (Join-Path $RepoRoot "manifests/image-payload.json")

    if ($IncludeLocalDrivers) {
        $Drivers = Join-Path $RepoRoot "drivers/local"
        if (-not (Test-Path $Drivers)) {
            throw "-IncludeLocalDrivers was requested, but drivers/local does not exist."
        }
        & dism.exe "/Image:$MountDirectory" /Add-Driver "/Driver:$Drivers" /Recurse
        if ($LASTEXITCODE -ne 0) {
            throw "DISM failed while adding local drivers."
        }
    }

    $Startnet = @"
wpeinit
wpeutil InitializeNetwork
if not exist X:\EffexorWinPE\reports mkdir X:\EffexorWinPE\reports
X:\EffexorWinPE\bin\effexorwinpe-collector.exe --output X:\EffexorWinPE\reports\initial.json
X:\EffexorWinPE\bin\effexorwinpe-agent.exe --input X:\EffexorWinPE\reports\initial.json --output X:\EffexorWinPE\reports\initial-diagnosis.json
cmd.exe
"@
    Set-Content -Path (Join-Path $MountDirectory "Windows/System32/startnet.cmd") -Value $Startnet -Encoding ASCII

    & dism.exe /Unmount-Image "/MountDir:$MountDirectory" /Commit
    if ($LASTEXITCODE -ne 0) {
        throw "DISM failed to commit boot.wim."
    }
    $Mounted = $false
}
finally {
    if ($Mounted) {
        & dism.exe /Unmount-Image "/MountDir:$MountDirectory" /Discard | Out-Null
    }
}

if (Test-Path $IsoPath) {
    Remove-Item -Force $IsoPath
}
$MediaArguments = @("/ISO", $WorkingDirectory, $IsoPath)
if ($BootEx) {
    $MediaArguments += "/bootex"
}
& $MakeWinPEMedia @MediaArguments
if ($LASTEXITCODE -ne 0) {
    throw "MakeWinPEMedia failed with exit code $LASTEXITCODE."
}

Write-Host "WinPE image ready: $IsoPath"
