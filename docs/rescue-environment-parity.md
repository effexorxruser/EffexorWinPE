# Rescue environment parity map

Research note for a Sergei Strelec–class technician rescue environment **without** copying Strelec media, proprietary programs, cracked utilities, or unlicensed binaries.

This document is a functional map: jobs-to-be-done, data EffexorWinPE already collects, gaps, automation candidates, dangerous actions, first-party scope, and where an external utility is justified. Implementation must stay behind explicit technician confirmation for mutations (`SECURITY.md`, ADR 0001).

Related artifacts:

- `manifests/tool-catalog.schema.json` — catalog contract
- `manifests/tool-catalog.json` — candidate tools and release profiles
- `docs/diagnostic-sources.md` — current collector sources
- `docs/roadmap.md` — M4 tool-catalog / repair milestones

## Non-goals

- Do not redistribute Strelec ISO contents or vendor installers without a recorded license and redistribution decision.
- Do not embed download URLs that fetch unapproved binaries into the image or CI.
- Do not expand diagnostic contracts, collector fields, or the gateway in this research track.
- Password/account “unlock” tooling that bypasses authentication remains policy-blocked unless a separate legal and safety review lands.

## Legend

| Term | Meaning |
| --- | --- |
| First-party | Owned EffexorWinPE code or Microsoft WinPE built-ins orchestrated by our shell |
| External utility | Third-party tool that needs license, checksum, and redistribution review before any profile |
| Automate | Scripted or typed operation with preview + confirmation |
| Dangerous | Can destroy data, brick firmware, or permanently alter boot/security state |

## Release profiles (summary)

Defined in `manifests/tool-catalog.json` → `profiles`:

| Profile | Intent |
| --- | --- |
| `minimal-diagnostics` | Read-only inventory and preflight only |
| `technician-standard` | Day-to-day repair: disks view, boot inspect/repair planning, file copy, wired network |
| `data-recovery` | Imaging and file-carving candidates (external, checksum-pinned) |
| `network-enabled` | Wired share/HTTP helpers and remote-assist *policy* hooks; no VPN secrets in image |
| `multiboot-extras` | Optional Linux live / MemTest-class entries on technician media (not inside WinPE WIM) |

---

## 1. Disks and partitions

### Technician jobs-to-be-done

- Identify every physical disk, bus type, size, and health signal before touching partitions.
- Map partitions, letters, GPT/MBR style, EFI/ESP, recovery, and BitLocker lock state.
- Shrink/create/delete/format only after backup plan and explicit confirmation.
- Assign letters and mount offline volumes for inspection.

### Data EffexorWinPE already collects

- Disk inventory (`number`, `friendly_name`, `bus_type`, `size_bytes`, `partition_style`, health/operational status, boot/system flags).
- Partition map (`disk_number`, `partition_number`, optional `drive_letter`, size, type/GPT type, active).
- Drive reliability counters when exposed (`temperature_celsius`, `wear_percent`, `power_on_hours`, read/write errors; nullable when absent).
- BitLocker inventory status and volume protection/lock fields when the provider works.

### Data gaps

- Volume labels, filesystem dirty flags, mount failures, and shadow-copy presence.
- Controller SMART beyond StorageReliabilityCounter (NVMe log pages, vendor SSD tools).
- LUN / multipath / USB enclosure pass-through quirks.
- Unmounted ESP contents beyond visible BCD path discovery.

### Automatable (with preview)

- Read-only disk/partition refresh into the report.
- Letter assignment for already-formatted volumes.
- Partition layout *plans* (diskpart/PowerShell scripts) shown before execution.

### Dangerous

- `clean` / `clean all`, deleting ESP or recovery partitions, converting MBR↔GPT with data loss, formatting wrong disk.

### First-party vs external

- **First-party:** collector inventory; future typed partition operations; shell presentation.
- **External:** advanced partition GUIs only if license/redistribution approved; prefer Microsoft `diskpart` / Storage cmdlets already in WinPE components.

---

## 2. Backup and restore

### Technician jobs-to-be-done

- Create sector or file-level images before repair.
- Restore images to replacement disks with size checks.
- Clone a failing disk to a healthy target with bad-sector handling.

### Data EffexorWinPE already collects

- Disk sizes and health counters that inform “image before repair” decisions.
- BitLocker lock state (imaging a locked volume is usually useless without unlock keys the collector deliberately does not store).

### Data gaps

- Existing backup inventory on technician USB.
- Image catalog metadata (hash, source disk serial redaction policy, compression).
- Free space on destination media.

### Automatable

- Preflight: “disk X looks failing → recommend image.”
- Guided imaging workflow with destination capacity checks and confirmation.

### Dangerous

- Restore to wrong disk; overwriting source; imaging without verifying target size.

### First-party vs external

- **First-party:** orchestration, destination policy, audit log.
- **External:** imaging engines (e.g. FOSS `dd`-class on a Linux live profile, or a reviewed Windows imaging utility). Nothing is bundled until checksum + license are recorded.

---

## 3. Data recovery

### Technician jobs-to-be-done

- Copy user files from a dying volume to external storage.
- Carve deleted files when the filesystem is damaged.
- Handle partially readable disks (timeouts, bad sectors).

### Data EffexorWinPE already collects

- Partition and letter map; BitLocker lock hints; reliability counters suggesting media failure.

### Data gaps

- File-tree samples (intentionally excluded for privacy).
- Deleted-entry indexes, photo/document signatures, RAID member maps.

### Automatable

- Read-only copy of selected trees after technician path review.
- Launch approved carver with output constrained to removable media.

### Dangerous

- Writes to the failing source; “repair filesystem” that reallocates metadata; running carvers that need GB of scratch on the wrong volume.

### First-party vs external

- **First-party:** path allowlists, export policy (see shell export rules), copy helpers.
- **External:** PhotoRec/TestDisk-class or similar FOSS tools under review; commercial recovery suites only with paid redistribution rights (usually **not** redistributable → technician-owned install, `download_mode: manual_official`).

---

## 4. Windows boot repair

### Technician jobs-to-be-done

- Explain why Windows does not start (BCD missing, wrong device, failed update, corrupt system files).
- Rebuild BCD, fix EFI boot files, run offline SFC/DISM when appropriate.
- Correlate detected installs with BCD entries.

### Data EffexorWinPE already collects

- Firmware mode (`uefi` / `bios` / `unknown`).
- Visible BCD store paths (`boot.bcd_stores[]`).
- Offline Windows installations (hive paths, normalized product/version).
- Disk boot/system flags and partitions.

### Data gaps

- Parsed BCD object inventory (bootmgr, `{default}`, device/osdevice, recovery sequence).
- Last BootID / boot failure counts from offline SYSTEM hive.
- Component store health and pending operations.
- Minidump / WER summary (roadmap M4).

### Automatable

- BCD *inspection* and correlation with installs (read-only).
- Planned `bootrec` / `bcdboot` / `bcdedit` sequences with preview.
- Offline SFC/DISM planning (roadmap).

### Dangerous

- Blind `bootrec /fixmbr` on GPT/UEFI systems; overwriting EFI; `bcdedit` deletes; DISM restore-health from untrusted media.

### First-party vs external

- **First-party:** inspection, typed repair operations, confirmation UI.
- **External:** usually unnecessary; use WinPE + ADK target tools. Third-party “one-click boot repair” suites are avoided unless license-clean and auditable.

---

## 5. Hardware diagnostics

### Technician jobs-to-be-done

- Confirm CPU/RAM presence and basic identity.
- Spot overheating or dying storage.
- Run memory tests and GPU/storage vendor diagnostics when needed.

### Data EffexorWinPE already collects

- System manufacturer/model, processor name/cores, total RAM.
- Firmware mode.
- Storage reliability counters and disk health strings.
- Network adapter names/status (no MAC/IP by default).

### Data gaps

- Per-DIMM SPD, ECC events, battery/ACPI, temperatures beyond disk counters.
- PCI device tree and driver bind failures.
- Extended SMART / NVMe health logs.

### Automatable

- Escalate to MemTest-class or vendor tools when counters look bad.
- Capture PCI/PnP inventory into a future report field (requires contract change — out of scope here).

### Dangerous

- Vendor firmware updaters; stress tests on failing PSUs; writing SMART “vendor specific” commands.

### First-party vs external

- **First-party:** inventory and triage rules.
- **External:** MemTest86+ / SystemRescue-style entries in `multiboot-extras`; OEM diagnostics under vendor license.

---

## 6. Network

### Technician jobs-to-be-done

- Get a wired link for updates, driver download, or gateway diagnosis.
- Map adapters and link state without collecting addresses by default.
- Optional share access to technician NAS.

### Data EffexorWinPE already collects

- Adapter `name` / `description` / normalized `status` (+ optional raw `status_code`).
- Privacy excludes network addresses, Wi-Fi profiles, and MACs by default.

### Data gaps

- DHCP lease success, DNS reachability, HTTP probe results (beyond boot init).
- Driver presence for specific NICs.
- Wi-Fi stack (explicitly not an MVP dependency per ADR 0001).

### Automatable

- `ipconfig` / route display after opt-in.
- Gateway upload remains separate (`--approve-upload` + device token).

### Dangerous

- Persisting Wi-Fi passwords on shared media; embedding VPN profiles or secrets in the image (forbidden).

### First-party vs external

- **First-party:** WinPE networking init, adapter inventory, gateway client.
- **External:** rare; prefer OS stack. Optional reviewed open-source share clients only if WinPE lacks needed protocols.

---

## 7. Malware scanning

### Technician jobs-to-be-done

- Scan offline Windows trees for known malware before repair.
- Quarantine or report findings without auto-deleting system files.

### Data EffexorWinPE already collects

- Offline install roots (scan targets).
- No file contents or AV results today.

### Data gaps

- Signature database freshness, scan reports, detections.

### Automatable

- Launch approved scanner against selected install root; import summary into session notes (not diagnostic contract until designed).

### Dangerous

- Aggressive remediation that breaks boot; false-positive deletion of drivers; outdated definitions giving false confidence.

### First-party vs external

- **First-party:** target selection, audit, no-auto-remediate policy.
- **External:** only engines with clear commercial/redistribution terms (many consumer AV tools **prohibit** ISO redistribution → `manual_official` / technician cache). Prefer portable scanners whose license explicitly allows rescue-media use after review.

---

## 8. Windows installation

### Technician jobs-to-be-done

- Apply a clean Windows image from official media.
- Preserve data partitions when reinstalling.
- Drivers injection for storage/NIC so setup can finish.

### Data EffexorWinPE already collects

- Existing installs and disk layout (to warn before wipe).
- Firmware mode for UEFI vs legacy setup expectations.

### Data gaps

- Official ISO hash verification UI.
- Product key / digital license status (sensitive; collect only if explicitly designed later).
- Driver pack matching for hardware IDs (driver manifest is empty pending reviewed sets).

### Automatable

- Guided wipe-and-load checklist; `dism /Apply-Image` with confirmation.
- Driver staging from `drivers/local` allowlisted sets.

### Dangerous

- Applying image to wrong disk; deleting data partitions; unattended scripts with leaked keys.

### First-party vs external

- **First-party:** orchestration and safety checks.
- **External:** Microsoft Windows install media only (technician-supplied). No pirated ISOs, no activation cracks.

---

## 9. Password / account recovery policy

### Technician jobs-to-be-done

- Help a **legitimate device owner** regain access using lawful methods.
- Prefer Microsoft account recovery, installation-media reset, or known recovery keys.

### Data EffexorWinPE already collects

- Offline install detection only.
- Usernames are excluded by default; SAM/hive password material is never collected.

### Data gaps (intentional)

- Local account hashes, PIN/NGC material, Autologon secrets — **out of scope**.

### Automatable

- Documented checklists: Microsoft account recovery links, BitLocker recovery key workflows *when the technician already possesses the key*, built-in Administrator enablement only on systems the owner authorizes.

### Dangerous / policy-blocked

- Hash dumping, offline password blanking utilities, bypass tools aimed at Windows login, and any Strelec-style “password reset” packs without legal review.
- Catalog entries for such tools must use `integration_status: policy_blocked` and must not ship.

### First-party vs external

- **First-party:** policy text in UI, refusal to automate bypasses.
- **External:** none for bypass. Official Microsoft recovery paths only.

---

## 10. Firmware and UEFI

### Technician jobs-to-be-done

- Know firmware mode and ESP location.
- Adjust boot order only when required.
- Apply OEM firmware updates from official packages when hardware is stuck.

### Data EffexorWinPE already collects

- `hardware.firmware_mode` / `boot.firmware_mode` from WinPE `PEFirmwareType`.
- Visible BCD stores; ESP not force-mounted solely for discovery.

### Data gaps

- Secure Boot state, Setup Mode, enrolled keys.
- OEM BIOS version strings (partially available via WMI on some systems — not yet contracted).
- Capsule update applicability.

### Automatable

- Read-only Secure Boot / firmware version probes (future contract).
- Launch OEM updater after hash verify — never unattended.

### Dangerous

- Interrupted BIOS flash; wrong firmware family; disabling Secure Boot without owner consent notes.

### First-party vs external

- **First-party:** mode detection, ESP/BCD awareness.
- **External:** OEM firmware utilities from official URLs only (`manual_official`, checksum required).

---

## 11. File management

### Technician jobs-to-be-done

- Browse volumes, copy out documents, edit config files carefully.
- Move reports to technician storage with path policy.

### Data EffexorWinPE already collects

- Partition letters and install roots.
- Shell export path evaluation (exclude Windows installs / fixed system volumes; prefer removable).

### Data gaps

- Rich file manager UX inside WinPE.
- Archive tools, editors, checksum utilities in payload.

### Automatable

- Export report/session bundles.
- Checksum verification helpers for approved downloads.

### Dangerous

- Editing `BCD` or registry hives without backup; recursive deletes on client disks.

### First-party vs external

- **First-party:** export pipeline, path policy, notepad/cmd via WinPE.
- **External:** optional FOSS file manager / 7-Zip-class archiver after license review.

---

## 12. Remote support

### Technician jobs-to-be-done

- Optional assisted session when the owner cannot sit with the machine.
- Keep credentials and desktop streams off the immutable image.

### Data EffexorWinPE already collects

- Nothing for remote desktop today.
- Gateway path is for **diagnosis upload**, not interactive remote control.

### Data gaps

- Consent workflow, session recording policy, supported remote tools list.

### Automatable

- Checklist: owner consent → network up → start approved assist tool from technician cache.

### Dangerous

- Unattended remote access binaries baked into public ISOs; hardcoded reverse tunnels; Telegram bots as the control plane (roadmap warns against this).

### First-party vs external

- **First-party:** consent + network readiness checks; keep gateway scoped to diagnosis.
- **External:** only reviewed remote-assist packages with redistribution rights; typically `technician_cache` / `manual_official`, never secrets in Git.

---

## Prioritized gap analysis

### P0 — required for a credible technician MVP

1. Parsed BCD inspection correlated with offline Windows installs (read-only).
2. Typed, previewable boot-repair operations (`bcdboot` / targeted `bcdedit`) behind confirmation.
3. Stable technician launcher grouping tools by **job**, not vendor (`roadmap` M2).
4. Checksum-pinned allowlist workflow for any third-party binary (`tool-catalog` + build fetch) — process, not binaries in Git.
5. Explicit password-bypass **policy block** in UI and catalog (`policy_blocked` entries as documentation only).

### P1 — high value after MVP boot repair

1. Offline SFC/DISM planning and execution with media validation.
2. Imaging/clone path for failing disks (external engine + first-party orchestration).
3. File copy / recovery workflow with export path policy reuse.
4. Minidump and selected event-log collection (contracted fields later).
5. Driver-set pipeline populated from reviewed OEM packages (`manifests/drivers.json`).

### P2 — broaden parity with Strelec-class media

1. Release profile packaging (`minimal-diagnostics` … `multiboot-extras`) in the build system.
2. Data-carving FOSS tools under `data-recovery` profile.
3. Optional malware scanner integration with no auto-remediation.
4. Secure Boot / firmware version inventory.
5. Multiboot entries for memory test and Linux live environments.

### Later

1. Full portable Windows environment for software that cannot run in WinPE.
2. Controlled update channel and signed releases for tool caches.
3. Remote-assist consent productization without turning chat apps into the core.
4. Commercial recovery/AV suites via technician-owned licenses only.
5. Wi-Fi and VPN client stories that never embed secrets in the image.

---

## Catalog maintenance rules

1. Every third-party tool needs `license`, `commercial_use`, `redistribution`, `official_url`, and `checksum_required: true` before `integration_status` may become `approved` or `shipped`.
2. `download_mode` of `build_time_fetch` or `technician_cache` is allowed only after redistribution is `allowed` or `restricted` with a recorded decision.
3. Prefer first-party + Microsoft WinPE components over GUI clones of Strelec menus.
4. Generated payloads and binaries stay out of Git (`AGENTS.md`).
5. New diagnostic fields still require `contracts/diagnostic-report.schema.json` updates — not done in this research PR.
