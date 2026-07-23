package present

import (
	"fmt"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/adapter"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/diagtext"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

// Screen identifiers for navigation.
const (
	ScreenOverview    = "overview"
	ScreenDiagnostics = "diagnostics"
	ScreenProgress    = "progress"
	ScreenSummary     = "summary"
	ScreenHardware    = "hardware"
	ScreenStorage     = "storage"
	ScreenBitLocker   = "bitlocker"
	ScreenWindows     = "windows"
	ScreenNetwork     = "network"
	ScreenAgent       = "agent"
	ScreenExport      = "export"
	ScreenJournal     = "journal"
	ScreenTools       = "tools"
	ScreenPower       = "power"
)

// NavItems returns ordered navigation screen ids.
func NavItems() []string {
	return []string{
		ScreenOverview,
		ScreenDiagnostics,
		ScreenProgress,
		ScreenSummary,
		ScreenHardware,
		ScreenStorage,
		ScreenBitLocker,
		ScreenWindows,
		ScreenNetwork,
		ScreenAgent,
		ScreenExport,
		ScreenJournal,
		ScreenTools,
		ScreenPower,
	}
}

// NavKey maps screen id to i18n key.
func NavKey(id string) string {
	switch id {
	case ScreenOverview:
		return "nav.overview"
	case ScreenDiagnostics:
		return "nav.diagnostics"
	case ScreenProgress:
		return "nav.progress"
	case ScreenSummary:
		return "nav.summary"
	case ScreenHardware:
		return "nav.hardware"
	case ScreenStorage:
		return "nav.storage"
	case ScreenBitLocker:
		return "nav.bitlocker"
	case ScreenWindows:
		return "nav.windows_installs"
	case ScreenNetwork:
		return "nav.network"
	case ScreenAgent:
		return "nav.agent"
	case ScreenExport:
		return "nav.export"
	case ScreenJournal:
		return "nav.journal"
	case ScreenTools:
		return "nav.tools"
	case ScreenPower:
		return "nav.power"
	default:
		return id
	}
}

// Render builds the text body for a screen.
func Render(b *i18n.Bundle, screen string, model viewmodel.AppModel) string {
	var lines []string
	heading, hint := screenHeading(b, screen)
	lines = append(lines, heading, hint, "")
	if model.MockMode && screen == ScreenOverview {
		lines = append(lines, b.T("msg.mock_mode"), "")
	}
	lines = append(lines, b.T("app.read_only_banner"), "")

	switch screen {
	case ScreenOverview:
		lines = append(lines, renderOverview(b, model)...)
	case ScreenDiagnostics:
		lines = append(lines, b.T("diagnostics.hint"), "", b.T("action.start_diagnostics"))
	case ScreenProgress:
		lines = append(lines, renderProgress(b, model)...)
	case ScreenSummary:
		lines = append(lines, renderSummary(b, model)...)
	case ScreenHardware:
		lines = append(lines, renderHardware(b, model)...)
	case ScreenStorage:
		lines = append(lines, renderStorage(b, model)...)
	case ScreenBitLocker:
		lines = append(lines, renderBitLocker(b, model)...)
	case ScreenWindows:
		lines = append(lines, renderWindows(b, model)...)
	case ScreenNetwork:
		lines = append(lines, renderNetwork(b, model)...)
	case ScreenAgent:
		lines = append(lines, renderAgent(b, model)...)
	case ScreenExport:
		lines = append(lines, renderExport(b, model)...)
	case ScreenJournal:
		lines = append(lines, renderJournal(b, model)...)
	case ScreenTools:
		lines = append(lines, b.T("tools.hint"), "", b.T("action.open_cmd"))
	case ScreenPower:
		lines = append(lines, b.T("power.hint"), "", b.T("action.reboot"), b.T("action.shutdown"))
	}
	return strings.Join(lines, "\n")
}

func screenHeading(b *i18n.Bundle, screen string) (string, string) {
	switch screen {
	case ScreenOverview:
		return b.T("overview.heading"), b.T("overview.hint")
	case ScreenDiagnostics:
		return b.T("diagnostics.heading"), b.T("diagnostics.hint")
	case ScreenProgress:
		return b.T("progress.heading"), b.T("progress.hint")
	case ScreenSummary:
		return b.T("summary.heading"), b.T("summary.hint")
	case ScreenHardware:
		return b.T("hardware.heading"), b.T("hardware.hint")
	case ScreenStorage:
		return b.T("storage.heading"), b.T("storage.hint")
	case ScreenBitLocker:
		return b.T("bitlocker.heading"), b.T("bitlocker.hint")
	case ScreenWindows:
		return b.T("windows.heading"), b.T("windows.hint")
	case ScreenNetwork:
		return b.T("network.heading"), b.T("network.hint")
	case ScreenAgent:
		return b.T("agent.heading"), b.T("agent.hint")
	case ScreenExport:
		return b.T("export.heading"), b.T("export.hint")
	case ScreenJournal:
		return b.T("journal.heading"), b.T("journal.hint")
	case ScreenTools:
		return b.T("tools.heading"), b.T("tools.hint")
	case ScreenPower:
		return b.T("power.heading"), b.T("power.hint")
	default:
		return screen, ""
	}
}

func renderOverview(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Overview.HasReport {
		return []string{b.T("msg.no_report")}
	}
	o := model.Overview
	return []string{
		fmt.Sprintf("%s: %s", b.T("app.brand"), b.T("label.diagnostics")),
		fmt.Sprintf("%s: %s", b.T("label.report_id"), o.ReportID),
		fmt.Sprintf("%s: %s", b.T("label.schema_version"), o.SchemaVersion),
		fmt.Sprintf("%s: %s", b.T("label.collected_at"), o.CollectedAt),
		fmt.Sprintf("%s: %s", b.T("label.firmware"), o.FirmwareMode),
		fmt.Sprintf("%s: %s", b.T("label.manufacturer"), o.Manufacturer),
		fmt.Sprintf("%s: %s", b.T("label.model"), o.Model),
		fmt.Sprintf("%s: %s", b.T("label.processor"), o.Processor),
		fmt.Sprintf("%s: %s", b.T("label.memory"), adapter.FormatBytes(o.MemoryBytes)),
		fmt.Sprintf("%s: %s", b.T("label.hostname"), o.Hostname),
		fmt.Sprintf("%s: %s", b.T("label.runtime"), o.Runtime),
		fmt.Sprintf("%s: %s=%d, %s=%d, %s=%d, %s=%d",
			b.T("label.checks"),
			b.T("label.checks_ok"), o.CheckOK,
			b.T("label.checks_warning"), o.CheckWarning,
			b.T("label.checks_error"), o.CheckError,
			b.T("label.checks_unknown"), o.CheckUnknown,
		),
	}
}

func renderProgress(b *i18n.Bundle, model viewmodel.AppModel) []string {
	p := model.Progress
	status := b.T(p.StatusKey)
	detail := p.Detail
	if strings.HasPrefix(detail, "msg.") || strings.HasPrefix(detail, "status.") {
		detail = b.T(detail)
	}
	lines := []string{
		fmt.Sprintf("%s: %s", b.T("label.progress"), status),
		fmt.Sprintf("%s: %d%%", b.T("label.progress"), p.Percent),
	}
	if detail != "" {
		lines = append(lines, detail)
	}
	if p.FriendlyError != "" {
		errText := p.FriendlyError
		if strings.HasPrefix(errText, "msg.") {
			errText = b.T(errText)
		}
		lines = append(lines, errText, b.T("msg.see_journal"))
	}
	return lines
}

func renderSummary(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Summary.HasReport {
		return []string{b.T("msg.no_report")}
	}
	lines := []string{b.T("label.results")}
	if model.Summary.Headline != "" {
		lines = append(lines, model.Summary.Headline, "")
	}
	for _, c := range model.Summary.Checks {
		localized, technical := diagtext.CheckSummary(b, c.ID, c.Summary)
		lines = append(lines, fmt.Sprintf("[%s] %s", b.T(c.StatusKey), localized))
		tech := c.ID
		if technical != "" && technical != localized {
			tech = c.ID + " — " + technical
		}
		lines = append(lines, "  "+b.T("msg.technical_details")+": "+tech)
	}
	return lines
}

func renderHardware(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Hardware.HasReport {
		return []string{b.T("msg.no_report")}
	}
	h := model.Hardware
	return []string{
		fmt.Sprintf("%s: %s", b.T("label.firmware"), h.FirmwareMode),
		fmt.Sprintf("%s: %s", b.T("label.manufacturer"), h.Manufacturer),
		fmt.Sprintf("%s: %s", b.T("label.model"), h.Model),
		fmt.Sprintf("%s: %s (%d / %d)", b.T("label.processor"), h.Processor, h.Cores, h.LogicalCPUs),
		fmt.Sprintf("%s: %s", b.T("label.memory"), adapter.FormatBytes(h.MemoryBytes)),
	}
}

func renderStorage(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Storage.HasReport {
		return []string{b.T("msg.no_report")}
	}
	na := b.T("label.na")
	lines := []string{b.T("label.storage_health"), ""}
	for _, d := range model.Storage.Disks {
		lines = append(lines, fmt.Sprintf("%s %d: %s | %s | %s | %s",
			b.T("label.disk"), d.Number, d.Name, d.BusType, adapter.FormatBytes(d.SizeBytes), d.HealthStatus))
	}
	lines = append(lines, "", b.T("label.smart"))
	for _, h := range model.Storage.Health {
		lines = append(lines,
			fmt.Sprintf("%s (%s)", h.Name, h.DeviceID),
			fmt.Sprintf("  %s: %s", b.T("label.temperature"), adapter.DisplayOptional(h.Temperature, na)),
			fmt.Sprintf("  %s: %s", b.T("label.wear"), adapter.DisplayOptional(h.WearPercent, na)),
			fmt.Sprintf("  %s: %s", b.T("label.power_on_hours"), adapter.DisplayOptional(h.PowerOnHours, na)),
			fmt.Sprintf("  %s: %s", b.T("label.read_errors"), adapter.DisplayOptional(h.ReadErrors, na)),
			fmt.Sprintf("  %s: %s", b.T("label.write_errors"), adapter.DisplayOptional(h.WriteErrors, na)),
		)
	}
	return lines
}

func renderBitLocker(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.BitLocker.HasReport {
		return []string{b.T("msg.no_report")}
	}
	lines := []string{b.T("label.bitlocker")}
	if model.BitLocker.StatusMessageKey != "" {
		lines = append(lines, b.T(model.BitLocker.StatusMessageKey))
	}
	if !model.BitLocker.VolumeCountKnown {
		lines = append(lines, b.T("msg.volume_count_unknown"))
		return lines
	}
	na := b.T("label.na")
	for _, v := range model.BitLocker.Volumes {
		lines = append(lines,
			fmt.Sprintf("%s: %s", b.T("label.mount_point"), v.MountPoint),
			fmt.Sprintf("  %s: %s", b.T("label.volume_status"), adapter.DisplayOptional(v.VolumeStatus, na)),
			fmt.Sprintf("  %s: %s", b.T("label.protection"), adapter.DisplayOptional(v.ProtectionStatus, na)),
			fmt.Sprintf("  %s: %s", b.T("label.lock"), adapter.DisplayOptional(v.LockStatus, na)),
			fmt.Sprintf("  %s: %s", b.T("label.encryption"), adapter.DisplayOptional(v.EncryptionMethod, na)),
		)
	}
	return lines
}

func renderWindows(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Windows.HasReport {
		return []string{b.T("msg.no_report")}
	}
	lines := []string{b.T("label.windows_installs")}
	if len(model.Windows.Installs) == 0 {
		key := model.Windows.EmptyKey
		if key == "" {
			key = "msg.no_windows_installs"
		}
		return append(lines, b.T(key))
	}
	for _, w := range model.Windows.Installs {
		lines = append(lines,
			fmt.Sprintf("%s: %s", b.T("label.root"), w.Root),
			fmt.Sprintf("  %s: %s", b.T("label.product_name"), w.ProductName),
			fmt.Sprintf("  %s: %s", b.T("label.edition"), w.EditionID),
			fmt.Sprintf("  %s: %s", b.T("label.build"), w.Build),
		)
	}
	return lines
}

func renderNetwork(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Network.HasReport {
		return []string{b.T("msg.no_report")}
	}
	lines := []string{b.T("label.network_adapters"), b.T("label.ethernet")}
	if model.Network.StatusMessageKey != "" {
		lines = append(lines, b.T(model.Network.StatusMessageKey))
	}
	for _, a := range model.Network.Adapters {
		lines = append(lines, fmt.Sprintf("%s: %s | %s | %s",
			b.T("label.adapter_name"), a.Name, a.Description, b.T(a.StatusKey)))
	}
	return lines
}

func renderAgent(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if !model.Agent.HasAssessment {
		return []string{b.T("msg.no_diagnosis")}
	}
	a := model.Agent
	headline := diagtext.Headline(b, a.Severity, a.Headline)
	lines := []string{
		fmt.Sprintf("%s: %s", b.T("label.results"), headline),
		fmt.Sprintf("%s: %s", b.T("label.severity"), diagtext.Severity(b, a.Severity)),
		fmt.Sprintf("%s: %s", b.T("label.session_id"), a.SessionID),
		"",
		b.T("label.findings"),
	}
	for _, f := range a.Findings {
		title := diagtext.FindingTitle(b, f.ID, f.Title)
		rationale := diagtext.FindingRationale(b, f.ID, f.Rationale)
		lines = append(lines, fmt.Sprintf("- [%s/%s] %s: %s",
			diagtext.Severity(b, f.Severity),
			diagtext.Confidence(b, f.Confidence),
			title, rationale))
		if f.Title != title || f.Rationale != rationale {
			lines = append(lines, "  "+b.T("msg.technical_details")+": "+f.ID)
		}
	}
	lines = append(lines, "", b.T("label.next_steps"))
	for _, s := range a.NextSteps {
		title := diagtext.StepTitle(b, s.ID, s.Title)
		rationale := diagtext.StepRationale(b, s.ID, s.Rationale)
		lines = append(lines, fmt.Sprintf("- %s — %s (%s)", title, rationale, diagtext.Risk(b, s.Risk)))
		if s.Operation != "" || s.ID != "" {
			lines = append(lines, "  "+b.T("msg.technical_details")+": "+s.ID+" / "+s.Operation)
		}
	}
	if len(a.Limitations) > 0 {
		lines = append(lines, "", b.T("label.limitations"))
		for _, lim := range a.Limitations {
			lines = append(lines, diagtext.Limitation(b, lim))
		}
	}
	return lines
}

func renderExport(b *i18n.Bundle, model viewmodel.AppModel) []string {
	e := model.Export
	lines := []string{
		fmt.Sprintf("%s: %s", b.T("label.export_target"), e.TargetDir),
		fmt.Sprintf("report: %s", e.ReportPath),
		fmt.Sprintf("diagnosis: %s", e.DiagnosisPath),
		fmt.Sprintf("session: %s", e.SessionPath),
		fmt.Sprintf("journal: %s", e.JournalPath),
		"",
		b.T("action.export_report"),
	}
	if e.LastMessage != "" {
		msg := e.LastMessage
		if strings.HasPrefix(msg, "msg.") {
			msg = b.T(msg)
		}
		lines = append(lines, "", msg)
	}
	return lines
}

func renderJournal(b *i18n.Bundle, model viewmodel.AppModel) []string {
	if len(model.Journal.Entries) == 0 {
		return []string{b.T("label.journal"), "(empty)"}
	}
	return append([]string{b.T("label.journal"), ""}, model.Journal.Entries...)
}
