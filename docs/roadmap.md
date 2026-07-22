# Roadmap

## M0 — repository foundation

- [x] Initialize repository rules and documentation.
- [x] Add versioned diagnostic report contract.
- [x] Add a dependency-free collector skeleton.
- [x] Add payload and WinPE build scripts.
- [x] Add CI for Go tests and Windows cross-build.

## M1 — bootable diagnostic MVP

- [ ] Build the first ISO on a Windows 11 ADK machine.
- [ ] Verify UEFI boot on a virtual machine and two physical PCs.
- [x] Implement inventory for disks, reliability counters, partitions, BitLocker state, firmware mode, memory, CPU, and network adapters.
- [x] Implement offline Windows version detection and visible BCD-store discovery.
- [ ] Validate every inventory source in the first WinPE build and record hardware-specific gaps.
- [ ] Inspect BCD entries and correlate them with detected Windows installations.
- [ ] Export JSON and readable HTML reports to technician storage.
- [ ] Add an explicit privacy preview before export.
- [x] Add an offline evidence-backed preflight with typed read-only next steps.

## M2 — technician launcher

- [ ] Build a keyboard-friendly launcher for 1366×768 and larger displays.
- [ ] Group tools by task rather than vendor name.
- [ ] Show offline/online state and report destination.
- [ ] Keep shell access available for advanced work.

## M3 — shared agent integration

- [ ] Implement the dedicated EffexorWinPE gateway endpoint.
- [ ] Implement device enrollment and revocation.
- [ ] Add report redaction and size limits.
- [ ] Return evidence, uncertainty, and proposed typed operations.
- [ ] Keep all mutations behind local confirmation.

## M4 — repair operations and tool catalog

- [ ] Add BCD inspection, then separately approved BCD repair.
- [ ] Add offline SFC/DISM planning and execution.
- [ ] Add minidump and event-log collection.
- [ ] Review third-party utility licenses and commercial-use restrictions.
- [ ] Add checksum-pinned downloads only for approved redistributable tools.

## Later

- Full portable Windows environment for software that cannot run reliably in WinPE.
- SystemRescue and MemTest86+ entries on the same technician drive.
- Controlled update channel and signed releases.
- Work-order linkage without turning Telegram into the system core.
