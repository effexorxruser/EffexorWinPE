# WinPE desktop-shell spike

Research and build scaffolding for an optional WinPE desktop shell. This does
**not** change the default release image.

## Profiles

| Profile | ISO name | Autostart | Third-party shell |
|---------|----------|-----------|-------------------|
| `minimal-shell` (default) | `EffexorWinPE-amd64.iso` | `effexorwinpe-shell.exe` then `cmd.exe` | none |
| `desktop-shell` (experimental) | `EffexorWinPE-amd64-desktop-shell.iso` | optional desktop shell, then Effexor GUI, then `cmd.exe` | only if staged under `third_party/winxshell/` |

Build:

```powershell
# Release path (unchanged)
.\build\Build-WinPE.ps1 -UILanguage ru-RU

# Experimental spike (requires staged binary + provenance)
.\build\Build-WinPE.ps1 -UILanguage ru-RU -ShellProfile desktop-shell
```

## License gate

Before any experimental ISO is shared outside the lab:

1. Confirm the binary was built from LGPL-2.1 sources (PExplorer `WinXShell_shellpart`
   or an equivalent documented fork).
2. Ship `COPYING.LGPL-2.1` / license notice beside the binary in the image payload.
3. Record source URL, commit/tag, build host, and SHA-256 in
   `third_party/winxshell/PROVENANCE.md`.
4. Do **not** redistribute closed-source WinXShell UI component packs.

See [ADR 0002](decisions/0002-winpe-desktop-shell-spike.md).

## Staging layout (gitignored binaries)

```
third_party/winxshell/
  PROVENANCE.md          # required, tracked
  LICENSE.LGPL-2.1.txt   # required when binary is staged
  WinXShell.exe          # gitignored; checksum-pinned in PROVENANCE.md
```

`Build-WinPE.ps1 -ShellProfile desktop-shell` fails closed if the binary or
provenance checksum is missing.

## Rollback

1. Rebuild with `-ShellProfile minimal-shell` (default), or
2. Boot the last known-good release ISO (`EffexorWinPE-amd64.iso`).

Emergency console: `cmd.exe` still starts after the GUI exits on both profiles.

## Validation checklist (lab)

- [ ] `minimal-shell` ISO still boots; Effexor GUI + Esc + cmd fallback work.
- [ ] `desktop-shell` ISO boots only with pinned binary present.
- [ ] Desktop shell starts under WinPE (`-winpe` or documented flag).
- [ ] Effexor GUI still launches and remains usable (mouse + keyboard).
- [ ] Record peak working-set / free RAM vs `minimal-shell` on the same VM.
- [ ] Screenshot set under `out/local-validation/desktop-shell-spike/` (not committed).
- [ ] Confirm no secrets or non-LGPL blobs entered the image.

## Memory / risk notes

- Desktop shells typically cost more RAM than kiosk Win32 UI alone; measure on a
  2–4 GB WinPE VM before recommending physical use.
- Mixing a third-party shell with our fullscreen Win32 UI can recreate input-focus
  issues; treat coexistence as part of the spike, not an assumed success.
- Opaque community binaries are rejected until provenance matches AGENTS.md
  third-party rules.
