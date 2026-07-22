# ANP Rescue

ANP Rescue is a technician-focused Windows recovery and diagnostics environment. It combines a reproducible WinPE build, a privacy-aware diagnostic collector, and a narrow client boundary for the shared ANP agent backend.

The project is intentionally not a repack of a third-party rescue ISO. The repository stores source code, manifests, and build scripts; Microsoft files, Windows images, drivers, and third-party utilities are supplied at build time.

## Current status

Bootstrap/MVP foundation:

- reproducible WinPE build skeleton;
- dependency-free Go diagnostic collector;
- versioned JSON diagnostic contract;
- payload and driver manifests;
- safety and secret-handling rules;
- initial CI workflow.

No distributable ISO is committed. The current image boots to a command prompt after creating an initial diagnostic report. A graphical launcher and backend connection come next.

## Repository layout

```text
build/                 Windows build and validation scripts
cmd/anp-collector/     WinPE diagnostic collector executable
contracts/             Versioned API and report schemas
docs/                  Architecture, roadmap, and decisions
drivers/               Documentation and local driver staging area
internal/              Collector and report implementation
manifests/              Auditable image contents
payload/ANP/            Files copied into X:\\ANP in WinPE
```

## Build prerequisites

On a Windows 11 x64 build machine install:

1. Windows ADK (Deployment Tools).
2. Matching Windows PE add-on for the ADK.
3. Go 1.24 or newer.
4. PowerShell 7 recommended; Windows PowerShell 5.1 is sufficient for the initial scripts.

Run from an elevated PowerShell session:

```powershell
Set-ExecutionPolicy -Scope Process Bypass
./build/Test-Repository.ps1
./build/Build-Payload.ps1
./build/Build-WinPE.ps1
```

The ISO is written to `out/anp-rescue-amd64.iso` by default.

On current ADK releases, pass `-BootEx` to create media signed for the UEFI 2023 CA. Keep the ordinary build available while testing older service hardware; this compatibility choice will become an explicit release profile before public distribution.

## Safety model

ANP Rescue separates inspection from repair:

1. Collectors read system state and produce a report.
2. The technician reviews exactly what may leave the device.
3. The client sends approved data to the ANP backend using a revocable device token.
4. The backend proposes bounded actions.
5. Any disk or OS mutation requires a local confirmation and is logged.

The OpenAI API key belongs only on the backend. It must never be placed in the ISO, payload, configuration committed to Git, or a client-side environment file.

## Licensing

The first-party source code is published under the MIT License; see `LICENSE`. That license does not grant redistribution rights for Microsoft components, Windows images, drivers, or third-party utilities. Those assets are not accepted into the image until their commercial-use and redistribution terms are recorded in the manifest.
