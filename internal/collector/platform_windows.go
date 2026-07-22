//go:build windows

package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

type inventoryPayload struct {
	Hardware diagnostics.Hardware `json:"hardware"`
	Storage  diagnostics.Storage  `json:"storage"`
	Errors   []string             `json:"errors"`
}

func collectPlatform() (diagnostics.Hardware, diagnostics.Storage, diagnostics.Boot, []diagnostics.Check) {
	payload, err := runPowerShellInventory()
	if err != nil {
		hardware, storage, boot := emptyPlatformReport()
		return hardware, storage, boot, []diagnostics.Check{{
			ID:      "platform.inventory",
			Status:  "error",
			Summary: err.Error(),
		}}
	}

	normalizeInventory(&payload)
	boot := diagnostics.Boot{
		FirmwareMode: payload.Hardware.FirmwareMode,
		BCDStores:    findBCDStores(),
	}
	inventoryStatus := "ok"
	inventorySuffix := ""
	if len(payload.Errors) > 0 {
		inventoryStatus = "warning"
		inventorySuffix = fmt.Sprintf("; %d source(s) unavailable", len(payload.Errors))
	}
	checks := []diagnostics.Check{{
		ID:      "platform.inventory",
		Status:  inventoryStatus,
		Summary: fmt.Sprintf("Collected %d disk(s), %d drive-health record(s), %d partition(s), and %d network adapter(s)%s", len(payload.Storage.Disks), len(payload.Storage.DriveHealth), len(payload.Storage.Partitions), len(payload.Hardware.NetworkAdapters), inventorySuffix),
	}}
	for index, sourceError := range payload.Errors {
		checks = append(checks, diagnostics.Check{
			ID:      fmt.Sprintf("platform.inventory.source.%d", index+1),
			Status:  "unknown",
			Summary: sourceError,
		})
	}
	if len(boot.BCDStores) == 0 {
		checks = append(checks, diagnostics.Check{
			ID:      "boot.bcd_stores",
			Status:  "warning",
			Summary: "No BCD store was found on a mounted volume",
		})
	} else {
		checks = append(checks, diagnostics.Check{
			ID:      "boot.bcd_stores",
			Status:  "ok",
			Summary: fmt.Sprintf("Found %d BCD store(s) on mounted volumes", len(boot.BCDStores)),
		})
	}
	return payload.Hardware, payload.Storage, boot, checks
}

func runPowerShellInventory() (inventoryPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", inventoryPowerShell)
	raw, err := command.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return inventoryPayload{}, fmt.Errorf("Windows inventory timed out after 30 seconds")
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return inventoryPayload{}, fmt.Errorf("PowerShell inventory failed: %s", strings.TrimSpace(string(exitError.Stderr)))
		}
		return inventoryPayload{}, fmt.Errorf("start PowerShell inventory: %w", err)
	}

	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	var payload inventoryPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return inventoryPayload{}, fmt.Errorf("decode PowerShell inventory: %w", err)
	}
	return payload, nil
}

func emptyPlatformReport() (diagnostics.Hardware, diagnostics.Storage, diagnostics.Boot) {
	hardware := diagnostics.Hardware{FirmwareMode: "unknown", NetworkAdapters: []diagnostics.NetworkAdapter{}}
	storage := diagnostics.Storage{Disks: []diagnostics.Disk{}, DriveHealth: []diagnostics.DriveHealth{}, Partitions: []diagnostics.Partition{}, BitLockerVolumes: []diagnostics.BitLockerVolume{}}
	boot := diagnostics.Boot{FirmwareMode: "unknown", BCDStores: findBCDStores()}
	return hardware, storage, boot
}

func normalizeInventory(payload *inventoryPayload) {
	if payload.Hardware.FirmwareMode == "" {
		payload.Hardware.FirmwareMode = "unknown"
	}
	if payload.Hardware.NetworkAdapters == nil {
		payload.Hardware.NetworkAdapters = []diagnostics.NetworkAdapter{}
	}
	if payload.Storage.Disks == nil {
		payload.Storage.Disks = []diagnostics.Disk{}
	}
	if payload.Storage.DriveHealth == nil {
		payload.Storage.DriveHealth = []diagnostics.DriveHealth{}
	}
	if payload.Storage.Partitions == nil {
		payload.Storage.Partitions = []diagnostics.Partition{}
	}
	if payload.Storage.BitLockerVolumes == nil {
		payload.Storage.BitLockerVolumes = []diagnostics.BitLockerVolume{}
	}
	if payload.Errors == nil {
		payload.Errors = []string{}
	}
}

const inventoryPowerShell = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$errors = @()
$firmwareMode = 'unknown'
$system = [ordered]@{}
$processor = [ordered]@{ cores = 0; logical_processors = 0 }
$memory = [ordered]@{ total_physical_bytes = 0 }
$networkAdapters = @()
$disks = @()
$driveHealth = @()
$partitions = @()
$bitLockerVolumes = @()

try {
  $firmwareValue = (Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control' -Name PEFirmwareType -ErrorAction Stop).PEFirmwareType
  if ($firmwareValue -eq 2) { $firmwareMode = 'uefi' }
  elseif ($firmwareValue -eq 1) { $firmwareMode = 'bios' }
} catch { $errors += 'Firmware mode is unavailable: ' + $_.Exception.Message }

try {
  $computer = Get-CimInstance Win32_ComputerSystem | Select-Object -First 1
  $system = [ordered]@{ manufacturer = [string]$computer.Manufacturer; model = [string]$computer.Model }
  $memory.total_physical_bytes = [uint64]$computer.TotalPhysicalMemory
} catch { $errors += 'Computer system inventory is unavailable: ' + $_.Exception.Message }

try {
  $processors = @(Get-CimInstance Win32_Processor)
  if ($processors.Count -gt 0) {
    $processor.name = [string]$processors[0].Name
    $processor.cores = [uint32](($processors | Measure-Object NumberOfCores -Sum).Sum)
    $processor.logical_processors = [uint32](($processors | Measure-Object NumberOfLogicalProcessors -Sum).Sum)
  }
} catch { $errors += 'Processor inventory is unavailable: ' + $_.Exception.Message }

try {
  $networkAdapters = @(Get-CimInstance Win32_NetworkAdapter | Where-Object { $_.PhysicalAdapter -eq $true } | ForEach-Object {
    [ordered]@{ name = [string]$_.Name; description = [string]$_.Description; status = [string]$_.NetConnectionStatus }
  })
} catch { $errors += 'Network adapter inventory is unavailable: ' + $_.Exception.Message }

try {
  $disks = @(Get-Disk | ForEach-Object {
    [ordered]@{
      number = [int]$_.Number; friendly_name = [string]$_.FriendlyName; bus_type = [string]$_.BusType
      size_bytes = [uint64]$_.Size; partition_style = [string]$_.PartitionStyle
      health_status = [string]$_.HealthStatus; operational_status = [string]$_.OperationalStatus
      is_boot = [bool]$_.IsBoot; is_system = [bool]$_.IsSystem
    }
  })
} catch { $errors += 'Disk inventory is unavailable: ' + $_.Exception.Message }

try {
  $driveHealth = @(Get-PhysicalDisk | ForEach-Object {
    $physicalDisk = $_
    $counter = $null
    try { $counter = $physicalDisk | Get-StorageReliabilityCounter -ErrorAction Stop } catch {}
    [ordered]@{
      device_id = [string]$physicalDisk.DeviceId; friendly_name = [string]$physicalDisk.FriendlyName
      media_type = [string]$physicalDisk.MediaType; health_status = [string]$physicalDisk.HealthStatus
      temperature_celsius = if ($null -ne $counter.Temperature) { [uint64]$counter.Temperature } else { $null }
      wear_percent = if ($null -ne $counter.Wear) { [uint64]$counter.Wear } else { $null }
      power_on_hours = if ($null -ne $counter.PowerOnHours) { [uint64]$counter.PowerOnHours } else { $null }
      read_errors_total = if ($null -ne $counter.ReadErrorsTotal) { [uint64]$counter.ReadErrorsTotal } else { $null }
      write_errors_total = if ($null -ne $counter.WriteErrorsTotal) { [uint64]$counter.WriteErrorsTotal } else { $null }
    }
  })
} catch { $errors += 'Storage reliability counters are unavailable: ' + $_.Exception.Message }

try {
  $partitions = @(Get-Partition | ForEach-Object {
    [ordered]@{
      disk_number = [int]$_.DiskNumber; partition_number = [int]$_.PartitionNumber; drive_letter = [string]$_.DriveLetter
      size_bytes = [uint64]$_.Size; type = [string]$_.Type; gpt_type = [string]$_.GptType; is_active = [bool]$_.IsActive
    }
  })
} catch { $errors += 'Partition inventory is unavailable: ' + $_.Exception.Message }

try {
  if (Get-Command Get-BitLockerVolume -ErrorAction SilentlyContinue) {
    $bitLockerVolumes = @(Get-BitLockerVolume | ForEach-Object {
      [ordered]@{
        mount_point = [string]$_.MountPoint; volume_status = [string]$_.VolumeStatus
        protection_status = [string]$_.ProtectionStatus; lock_status = [string]$_.LockStatus
        encryption_method = [string]$_.EncryptionMethod
      }
    })
  } else {
    $conversionNames = @('FullyDecrypted', 'FullyEncrypted', 'EncryptionInProgress', 'DecryptionInProgress', 'EncryptionPaused', 'DecryptionPaused')
    $protectionNames = @('Off', 'On', 'Unknown')
    $lockNames = @('Unlocked', 'Locked')
    $encryptionNames = @('None', 'AES_128_WITH_DIFFUSER', 'AES_256_WITH_DIFFUSER', 'AES_128', 'AES_256', 'HardwareEncryption', 'XTS_AES_128', 'XTS_AES_256')
    $encryptableVolumes = @(Get-CimInstance -Namespace 'root/CIMV2/Security/MicrosoftVolumeEncryption' -ClassName Win32_EncryptableVolume)
    $bitLockerVolumes = @($encryptableVolumes | ForEach-Object {
      $conversion = Invoke-CimMethod -InputObject $_ -MethodName GetConversionStatus
      $protection = Invoke-CimMethod -InputObject $_ -MethodName GetProtectionStatus
      $lock = Invoke-CimMethod -InputObject $_ -MethodName GetLockStatus
      $mountPoint = [string]$_.DriveLetter
      if (-not $mountPoint) { $mountPoint = [string]$_.PersistentVolumeID }
      $volumeStatus = if ($conversion.ConversionStatus -lt $conversionNames.Count) { $conversionNames[$conversion.ConversionStatus] } else { [string]$conversion.ConversionStatus }
      $protectionStatus = if ($protection.ProtectionStatus -lt $protectionNames.Count) { $protectionNames[$protection.ProtectionStatus] } else { [string]$protection.ProtectionStatus }
      $lockStatus = if ($lock.LockStatus -lt $lockNames.Count) { $lockNames[$lock.LockStatus] } else { [string]$lock.LockStatus }
      $encryptionMethod = if ($conversion.EncryptionMethod -lt $encryptionNames.Count) { $encryptionNames[$conversion.EncryptionMethod] } else { [string]$conversion.EncryptionMethod }
      [ordered]@{
        mount_point = $mountPoint; volume_status = $volumeStatus; protection_status = $protectionStatus
        lock_status = $lockStatus; encryption_method = $encryptionMethod
      }
    })
  }
} catch { $errors += 'BitLocker inventory is unavailable: ' + $_.Exception.Message }

[ordered]@{
  hardware = [ordered]@{
    firmware_mode = $firmwareMode; system = $system; processor = $processor; memory = $memory; network_adapters = $networkAdapters
  }
  storage = [ordered]@{ disks = $disks; drive_health = $driveHealth; partitions = $partitions; bitlocker_volumes = $bitLockerVolumes }
  errors = $errors
} | ConvertTo-Json -Depth 7 -Compress
`
