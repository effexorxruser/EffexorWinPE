# Repository guidance

EffexorWinPE is a personal repair and diagnostics environment. Treat client data and disks as safety-critical.

## Invariants

- Inspection is the default. Mutating repair operations must be explicit, previewable, and confirmed by the technician.
- Never embed OpenAI keys, VPN credentials, device tokens, client data, or private certificates in the image or repository.
- The rescue client may perform bounded deterministic preflight checks, while model-backed reasoning remains behind the EffexorWinPE agent gateway and its narrow versioned API.
- New diagnostic fields must be documented in `contracts/diagnostic-report.schema.json`.
- Third-party binaries, drivers, and fonts require a recorded source, checksum, license, and redistribution decision before inclusion.
- Generated images and payload binaries stay out of Git.

## Validation

- Run `go test ./...` after Go changes.
- Run `build/Test-Repository.ps1` on Windows before building an image.
- Build WinPE only on Windows with the supported Windows ADK and WinPE add-on installed.
