# Localization

EffexorWinPE ships two UI catalogs:

- [`locales/ru-RU.json`](../locales/ru-RU.json) — default
- [`locales/en-US.json`](../locales/en-US.json) — fallback

They are embedded through [`locales/embed.go`](../locales/embed.go) and resolved by
[`internal/shell/i18n`](../internal/shell/i18n).

## Application UI rules

- Stable string keys (for example `label.diagnostics`, `action.export_report`)
- No user-visible literals inside Win32 command handlers
- Variables and code identifiers remain English
- Default locale is `ru-RU`
- Missing keys fall back to `en-US`, then to the key itself
- Technical terms stay untranslated: SMART, BitLocker, BCD, UEFI, BIOS, WinPE, JSON, Ethernet
- Branding is EffexorWinPE only (no ANP)

Required Russian terminology includes:

- Диагностика
- Сбор данных
- Результаты
- Обнаруженные установки Windows
- Состояние накопителей
- Шифрование BitLocker
- Сетевые адаптеры
- Подключение Ethernet
- Источник данных недоступен
- Количество томов неизвестно
- Экспортировать отчёт
- Открыть журнал
- Открыть командную строку
- Перезагрузить компьютер
- Завершить работу

Win32 APIs receive UTF-16 strings. Helpers live in `internal/shell/i18n`
(`EncodeUTF16` / `DecodeUTF16`) and `golang.org/x/sys/windows`.

### Tests

- Key parity between `ru-RU` and `en-US`
- Fallback when a primary key is missing
- UTF-16 round-trip for Cyrillic
- Absence of ANP in catalogs and mock assets

### CLI

```text
effexorwinpe-shell.exe --locale ru-RU
effexorwinpe-shell.exe --locale en-US --mock
```

## WinPE OS language packs

Application localization does **not** require WinPE language packs. WinPE packs
only affect the built-in Windows PE UI (for example `cmd.exe` messages).

Helper script:

[`build/Add-WinPELanguage.ps1`](../build/Add-WinPELanguage.ps1)

It:

- Resolves or accepts a Windows ADK path
- Requires a mounted WinPE image directory
- Defaults to `ru-RU`
- Uses official ADK WinPE language pack CABs (not stored in this repository)
- Validates CAB presence before DISM
- Adds the base `lp.cab` plus matching localized packs for Optional Components
- Sets system/UI locale to `ru-RU`
- Adds Russian and English keyboard layouts, keeping English as fallback
- Supports `-DryRun`

Example with `Build-WinPE.ps1` (while `boot.wim` is mounted):

```powershell
.\build\Build-WinPE.ps1 -UILanguage ru-RU
.\build\Build-WinPE.ps1 -SkipOSLanguagePack
```

- `-UILanguage` (default `ru-RU`) is passed to `Add-WinPELanguage.ps1`
- `-SkipOSLanguagePack` skips ADK language packs; the shell UI stays Russian via catalogs

Standalone helper usage:

```powershell
.\build\Add-WinPELanguage.ps1 -MountDirectory C:\WinPE\mount -Locale ru-RU -DryRun
.\build\Add-WinPELanguage.ps1 -MountDirectory C:\WinPE\mount -Locale ru-RU
```

Application localization does not require WinPE language packs. The shell remains
Russian via embedded catalogs even when `-SkipOSLanguagePack` is used.
