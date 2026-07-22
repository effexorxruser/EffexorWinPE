# Diagnostic sources

The collector treats every Windows diagnostic provider as optional. A missing WMI namespace, PowerShell cmdlet, driver, or controller capability must reduce confidence; it must not abort the complete report or fabricate a healthy value.

| Report area | Primary source | Fallback or limitation |
| --- | --- | --- |
| System, CPU, memory, network adapters | CIM/WMI | Reported as unavailable if the WinPE WMI provider is missing |
| Disks and partitions | Storage PowerShell cmdlets | Requires WinPE StorageWMI components and a working storage driver |
| Drive reliability | `Get-StorageReliabilityCounter` | Fields are omitted when the device or controller does not expose them |
| BitLocker | `Get-BitLockerVolume` | Falls back to the BitLocker WMI provider and typed method results |
| Firmware mode | WinPE `PEFirmwareType` registry value | `unknown` outside WinPE or when the value is unavailable |
| Offline Windows version | Temporarily loaded offline SOFTWARE hive | Version stays absent if the hive is locked, damaged, or unreadable |
| Boot configuration | Visible BIOS/UEFI BCD files | Unmounted EFI partitions are not modified just to discover a BCD store |

Serial numbers, MAC addresses, IP addresses, usernames, Wi-Fi profiles, file contents, and recovery keys are not collected. Local path and hostname fields still require technician review before any future upload.
