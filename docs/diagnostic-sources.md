# Diagnostic sources

The collector treats every Windows diagnostic provider as optional. A missing WMI namespace, PowerShell cmdlet, driver, or controller capability must reduce confidence; it must not abort the complete report or fabricate a healthy value.

| Report area | Primary source | Fallback or limitation |
| --- | --- | --- |
| System, CPU, memory, network adapters | CIM/WMI | Reported as unavailable if the WinPE WMI provider is missing. `status` is a stable NetConnectionStatus name; `status_code` keeps the raw numeric value |
| Disks and partitions | Storage PowerShell cmdlets | Requires WinPE StorageWMI components and a working storage driver |
| Drive reliability | `Get-StorageReliabilityCounter` | Optional counters (`temperature_celsius`, `wear_percent`, `power_on_hours`, `read_errors_*`) are JSON `null` when the device or controller does not expose them. Real zeros remain zeros |
| BitLocker | `Get-BitLockerVolume` | Falls back to `Win32_EncryptableVolume`. `bitlocker_inventory.status` is `ok`, `partial`, or `unavailable`. When unavailable, `bitlocker_volumes` is `null` rather than `[]` |
| Firmware mode | WinPE `PEFirmwareType` registry value | `unknown` outside WinPE or when the value is unavailable |
| Offline Windows version | Temporarily loaded offline SOFTWARE hive | Stores `raw_product_name` plus a normalized `product_name` (client builds ≥ 22000 rewrite legacy "Windows 10 *" names). The running WinPE/SystemRoot install is never listed as an offline installation |
| Boot configuration | Visible BIOS/UEFI BCD files | Unmounted EFI partitions are not modified just to discover a BCD store |

Serial numbers, MAC addresses, IP addresses, usernames, Wi-Fi profiles, file contents, and recovery keys are not collected. Local path and hostname fields still require technician review before any future upload.

## Schema 1.3.0 compatibility notes

Diagnostic report `schema_version` is now `1.3.0`. Additive changes relative to `1.2.0`:

- `hardware.network_adapters[].status_code` (optional raw NetConnectionStatus integer)
- `hardware.network_adapters[].status` normalized to stable names (`media_disconnected`, `connected`, …, or `unknown_<code>`)
- `storage.bitlocker_inventory` (`status`, optional `error`)
- `storage.bitlocker_volumes` may be `null` when the BitLocker provider is unavailable
- `storage.drive_health` optional counters are explicitly nullable
- `windows_installations[].version.raw_product_name` preserves the registry ProductName

Readers that already tolerated unknown fields can accept 1.3.0 reports after updating the expected `schema_version`.

### Reading legacy `1.2.0` reports

`diagnostics.DecodeReportJSON` (used by the agent and by `Report.UnmarshalJSON` for the gateway) accepts `1.2.0` only through an explicit migration to `1.3.0`:

- strict `1.3.0` validation is unchanged for current reports;
- a legacy empty `bitlocker_volumes` array without availability metadata becomes `bitlocker_inventory.status=unavailable` and `bitlocker_volumes=null` (never a trusted `ok`);
- non-empty legacy BitLocker volumes migrate to `status=ok`;
- network `status` codes and Windows product names are normalized during migration.

JSON Schema `allOf` conditionals on `storage` require `bitlocker_volumes` to be `null` when status is `unavailable`, and an array when status is `ok` or `partial`.
