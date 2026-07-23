# EffexorWinPE

EffexorWinPE is a personal technician-focused Windows recovery and diagnostics environment. It combines a reproducible WinPE build, a privacy-aware diagnostic collector, an offline diagnostic preflight, and a narrow client boundary for its optional AI agent gateway.

The project is intentionally not a repack of a third-party rescue ISO. The repository stores source code, manifests, and build scripts; Microsoft files, Windows images, drivers, and third-party utilities are supplied at build time.

## Current status

Bootstrap/MVP foundation:

- reproducible WinPE build skeleton;
- dependency-free Go diagnostic collector;
- versioned JSON diagnostic contract (`diagnostic-report` 1.3.0);
- read-only hardware, storage reliability/SMART counters, BitLocker availability, firmware, BCD, and offline Windows inventory (running WinPE excluded);
- offline evidence-backed diagnostic preflight with confidence, follow-up questions, limitations, and typed read-only next steps;
- resumable diagnostic sessions with technician symptoms, answers, and a compact audit timeline;
- an opt-in HTTPS client for the model-backed gateway, with a removable device token and a separate upload approval flag;
- an authenticated asynchronous gateway with strict model output, official-domain web retrieval, source capture, and a read-only operation boundary;
- payload and driver manifests;
- safety and secret-handling rules;
- initial CI workflow.

No distributable ISO is committed. The current image boots to a command prompt after creating an initial diagnostic report, offline preflight assessment, and resumable diagnostic-session file. Hardware collection is best-effort: unavailable WinPE components become explicit `unknown` checks instead of aborting the report. The preflight cannot execute repairs and never treats missing evidence as proof that a device is healthy. The first real ADK/WinPE boot and a graphical launcher still need validation.

## Repository layout

```text
build/                 Windows build and validation scripts
cmd/effexorwinpe-collector/  WinPE diagnostic collector executable
cmd/effexorwinpe-agent/      Offline preflight, session, and gateway client
cmd/effexorwinpe-gateway/    Server-side authenticated model gateway
contracts/             Versioned API and report schemas
deploy/gateway/         Container and deployment example; no secrets
docs/                  Architecture, roadmap, and decisions
drivers/               Documentation and local driver staging area
internal/              Collector and report implementation
manifests/              Auditable image contents
payload/EffexorWinPE/         Files copied into X:\\EffexorWinPE in WinPE
```

The image payload is copied from the closed allowlist in `manifests/image-payload.json`.
Files merely present under `payload/EffexorWinPE` are never included automatically, so
local reports, credentials, and other ignored build-host data cannot leak into an ISO.

## Diagnostic session and agent

The automatic boot flow creates `initial-diagnosis-session.json` beside the initial assessment. Resume it from the WinPE command prompt to add context:

```powershell
X:\EffexorWinPE\bin\effexorwinpe-agent.exe `
  --input X:\EffexorWinPE\reports\initial.json `
  --output X:\EffexorWinPE\reports\initial-diagnosis.json `
  --session X:\EffexorWinPE\reports\initial-diagnosis-session.json `
  --interactive
```

For scripting, repeat `--symptom` and use `--answer question-id=value`. Russian `да`, `нет`, and `не знаю` are normalized for yes/no questions.

Online submission is disabled unless the technician supplies an HTTPS gateway URL, an external token file, and `--approve-upload` together. This is an explicit upload of the report plus session context; symptom free text can contain personal data and must be reviewed first. The gateway strips the hostname again, sends model requests with storage disabled, constrains web retrieval to reviewed official domains, and rejects invented evidence paths, source URLs, or operations. See [`docs/gateway.md`](docs/gateway.md).

## Build prerequisites

On a Windows 11 x64 build machine install:

1. Windows ADK (Deployment Tools) from the [official Microsoft download page](https://learn.microsoft.com/windows-hardware/get-started/adk-install).
2. The matching Windows PE add-on for that ADK. For an `amd64` image, choose an ADK release that supports x64 Windows 10/11 rather than an Arm64-only release.
3. Go 1.24 or newer.
4. PowerShell 7 recommended; Windows PowerShell 5.1 is sufficient for the initial scripts.

Start **Deployment and Imaging Tools Environment** as administrator, then launch `powershell` (or `pwsh`) from that window so the ADK environment is inherited. From the repository root run:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
.\build\Test-Repository.ps1
.\build\Build-WinPE.ps1
```

`Build-WinPE.ps1` builds the payload automatically. The ISO is written to `out/EffexorWinPE-amd64.iso` by default. Record its size and digest before testing:

```powershell
Get-Item .\out\EffexorWinPE-amd64.iso |
    Select-Object FullName, Length, LastWriteTime
Get-FileHash .\out\EffexorWinPE-amd64.iso -Algorithm SHA256
```

On current ADK releases, pass `-BootEx` to create media signed for the UEFI 2023 CA. Keep the ordinary build available while testing older service hardware; this compatibility choice will become an explicit release profile before public distribution.

## First boot smoke test

Test the ISO in a VM before writing it to a USB drive. Boot waits for WinPE initialization, initializes wired networking, collects the system report, creates the offline assessment and session, and then opens `cmd.exe`.

At the prompt, verify the expected files and network state:

```batch
dir X:\EffexorWinPE\reports
ipconfig /all
```

Before shutting down, copy the JSON files to a writable attached disk. Use `diskpart` followed by `list volume` and `exit` to identify its drive letter; do not guess the letter. For example, if it is `E:`:

```batch
mkdir E:\EffexorWinPE-reports
copy X:\EffexorWinPE\reports\*.json E:\EffexorWinPE-reports\
```

## Safety model

EffexorWinPE separates inspection from repair:

1. Collectors read system state and produce a report.
2. The technician reviews exactly what may leave the device.
3. The client sends approved data to the EffexorWinPE agent gateway using a revocable device token.
4. The gateway returns sourced findings and bounded read-only diagnostic actions.
5. Any disk or OS mutation requires a local confirmation and is logged.

The OpenAI API key belongs only on the backend. It must never be placed in the ISO, payload, configuration committed to Git, or a client-side environment file.

## Licensing

The first-party source code is published under the MIT License; see `LICENSE`. That license does not grant redistribution rights for Microsoft components, Windows images, drivers, or third-party utilities. Those assets are not accepted into the image until their commercial-use and redistribution terms are recorded in the manifest.
