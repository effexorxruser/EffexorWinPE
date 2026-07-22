package collector

const inventoryPowerShell = `
$ErrorActionPreference = 'Stop'
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8

function Get-NullableUInt64 {
  param($Value)
  if ($null -eq $Value) { return $null }
  try { return [uint64]$Value } catch { return $null }
}

function Get-NullableString {
  param($Value)
  if ($null -eq $Value) { return $null }
  return [string]$Value
}

function Get-NamedStatus {
  param(
    [object[]]$Names,
    $Index
  )
  if ($null -eq $Index) { return $null }
  try {
    $numeric = [int]$Index
    if ($numeric -ge 0 -and $numeric -lt $Names.Count) {
      return [string]$Names[$numeric]
    }
    return [string]$Index
  } catch {
    return [string]$Index
  }
}

$errors = @()
$firmwareMode = 'unknown'
$system = [ordered]@{}
$processor = [ordered]@{ cores = 0; logical_processors = 0 }
$memory = [ordered]@{ total_physical_bytes = 0 }
$networkAdapters = @()
$disks = @()
$driveHealth = @()
$partitions = @()
$bitLockerVolumes = $null
$bitLockerInventory = [ordered]@{ status = 'unavailable'; error = 'BitLocker inventory was not attempted' }

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
    $code = $null
    if ($null -ne $_.NetConnectionStatus) {
      try { $code = [int]$_.NetConnectionStatus } catch { $code = $null }
    }
    [ordered]@{
      name = [string]$_.Name
      description = [string]$_.Description
      status = if ($null -ne $code) { [string]$code } else { '' }
      status_code = $code
    }
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
    try { $counter = $physicalDisk | Get-StorageReliabilityCounter -ErrorAction Stop } catch { $counter = $null }
    $temperature = $null
    $wear = $null
    $powerOnHours = $null
    $readErrors = $null
    $writeErrors = $null
    if ($null -ne $counter) {
      $temperature = Get-NullableUInt64 $counter.Temperature
      $wear = Get-NullableUInt64 $counter.Wear
      $powerOnHours = Get-NullableUInt64 $counter.PowerOnHours
      $readErrors = Get-NullableUInt64 $counter.ReadErrorsTotal
      $writeErrors = Get-NullableUInt64 $counter.WriteErrorsTotal
    }
    [ordered]@{
      device_id = [string]$physicalDisk.DeviceId
      friendly_name = [string]$physicalDisk.FriendlyName
      media_type = [string]$physicalDisk.MediaType
      health_status = [string]$physicalDisk.HealthStatus
      temperature_celsius = $temperature
      wear_percent = $wear
      power_on_hours = $powerOnHours
      read_errors_total = $readErrors
      write_errors_total = $writeErrors
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
    $partial = $false
    $collected = New-Object System.Collections.Generic.List[object]
    foreach ($volume in @(Get-BitLockerVolume)) {
      $mountPoint = Get-NullableString $volume.MountPoint
      $volumeStatus = Get-NullableString $volume.VolumeStatus
      $protectionStatus = Get-NullableString $volume.ProtectionStatus
      $lockStatus = Get-NullableString $volume.LockStatus
      $encryptionMethod = Get-NullableString $volume.EncryptionMethod
      if (-not $mountPoint) {
        $mountPoint = 'unknown'
        $partial = $true
      }
      if ($null -eq $volumeStatus -or $null -eq $protectionStatus -or $null -eq $lockStatus -or $null -eq $encryptionMethod) {
        $partial = $true
      }
      $collected.Add([ordered]@{
        mount_point = $mountPoint
        volume_status = $volumeStatus
        protection_status = $protectionStatus
        lock_status = $lockStatus
        encryption_method = $encryptionMethod
      }) | Out-Null
    }
    $bitLockerVolumes = @($collected)
    if ($partial) {
      $bitLockerInventory = [ordered]@{ status = 'partial'; error = 'One or more BitLocker volume fields were incomplete' }
    } else {
      $bitLockerInventory = [ordered]@{ status = 'ok' }
    }
  } else {
    $conversionNames = @('FullyDecrypted', 'FullyEncrypted', 'EncryptionInProgress', 'DecryptionInProgress', 'EncryptionPaused', 'DecryptionPaused')
    $protectionNames = @('Off', 'On', 'Unknown')
    $lockNames = @('Unlocked', 'Locked')
    $encryptionNames = @('None', 'AES_128_WITH_DIFFUSER', 'AES_256_WITH_DIFFUSER', 'AES_128', 'AES_256', 'HardwareEncryption', 'XTS_AES_128', 'XTS_AES_256')
    $encryptableVolumes = @()
    try {
      $encryptableVolumes = @(Get-CimInstance -Namespace 'root/CIMV2/Security/MicrosoftVolumeEncryption' -ClassName Win32_EncryptableVolume -ErrorAction Stop)
    } catch {
      throw
    }
    $partial = $false
    $collected = New-Object System.Collections.Generic.List[object]
    foreach ($volume in $encryptableVolumes) {
      try {
        $conversion = $null
        $protection = $null
        $lock = $null
        try { $conversion = Invoke-CimMethod -InputObject $volume -MethodName GetConversionStatus -ErrorAction Stop } catch { $partial = $true }
        try { $protection = Invoke-CimMethod -InputObject $volume -MethodName GetProtectionStatus -ErrorAction Stop } catch { $partial = $true }
        try { $lock = Invoke-CimMethod -InputObject $volume -MethodName GetLockStatus -ErrorAction Stop } catch { $partial = $true }

        $mountPoint = $null
        if ($null -ne $volume.DriveLetter -and [string]$volume.DriveLetter -ne '') {
          $mountPoint = [string]$volume.DriveLetter
        } elseif ($null -ne $volume.PersistentVolumeID -and [string]$volume.PersistentVolumeID -ne '') {
          $mountPoint = [string]$volume.PersistentVolumeID
        } else {
          $mountPoint = 'unknown'
          $partial = $true
        }

        $conversionStatus = $null
        $encryptionMethodValue = $null
        if ($null -ne $conversion) {
          if ($null -ne $conversion.ConversionStatus) { $conversionStatus = $conversion.ConversionStatus }
          if ($null -ne $conversion.EncryptionMethod) { $encryptionMethodValue = $conversion.EncryptionMethod }
        }
        $protectionStatus = $null
        if ($null -ne $protection -and $null -ne $protection.ProtectionStatus) { $protectionStatus = $protection.ProtectionStatus }
        $lockStatus = $null
        if ($null -ne $lock -and $null -ne $lock.LockStatus) { $lockStatus = $lock.LockStatus }

        $collected.Add([ordered]@{
          mount_point = $mountPoint
          volume_status = Get-NamedStatus -Names $conversionNames -Index $conversionStatus
          protection_status = Get-NamedStatus -Names $protectionNames -Index $protectionStatus
          lock_status = Get-NamedStatus -Names $lockNames -Index $lockStatus
          encryption_method = Get-NamedStatus -Names $encryptionNames -Index $encryptionMethodValue
        }) | Out-Null
      } catch {
        $partial = $true
      }
    }
    $bitLockerVolumes = @($collected)
    if ($partial) {
      $bitLockerInventory = [ordered]@{ status = 'partial'; error = 'One or more BitLocker volume fields were incomplete' }
    } else {
      $bitLockerInventory = [ordered]@{ status = 'ok' }
    }
  }
} catch {
  # BitLocker failures are reported only through bitlocker_inventory / storage.bitlocker_inventory.
  # Do not also append a generic platform.inventory.source error for the same provider.
  $bitLockerVolumes = $null
  $bitLockerInventory = [ordered]@{ status = 'unavailable'; error = $_.Exception.Message }
}

[ordered]@{
  hardware = [ordered]@{
    firmware_mode = $firmwareMode; system = $system; processor = $processor; memory = $memory; network_adapters = $networkAdapters
  }
  storage = [ordered]@{
    disks = $disks
    drive_health = $driveHealth
    partitions = $partitions
    bitlocker_volumes = $bitLockerVolumes
    bitlocker_inventory = $bitLockerInventory
  }
  errors = $errors
} | ConvertTo-Json -Depth 7 -Compress
`
