package adapter

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/effexorxruser/EffexorWinPE/internal/shell/viewmodel"
)

// FromReport maps a diagnostic report into screen view-models.
// UI code should depend on viewmodel types, not diagnostics structs.
func FromReport(report diagnostics.Report, mock bool) viewmodel.AppModel {
	model := viewmodel.AppModel{MockMode: mock}
	model.Overview = overviewFrom(report, mock)
	model.Summary = summaryFrom(report)
	model.Hardware = hardwareFrom(report)
	model.Storage = storageFrom(report)
	model.BitLocker = bitlockerFrom(report)
	model.Windows = windowsFrom(report)
	model.Network = networkFrom(report)
	model.Progress = viewmodel.ProgressScreen{
		Phase:     "idle",
		StatusKey: "status.idle",
		Percent:   0,
	}
	return model
}

// ApplyAssessment merges diagnosis and optional session into the model.
func ApplyAssessment(model *viewmodel.AppModel, assessment diagnosis.Assessment, sess *session.Session) {
	if model == nil {
		return
	}
	screen := viewmodel.AgentScreen{
		HasAssessment: true,
		Mode:          assessment.Mode,
		Headline:      assessment.Summary.Headline,
		Severity:      assessment.Summary.HighestSeverity,
		FindingCount:  assessment.Summary.FindingCount,
		Limitations:   append([]string{}, assessment.Limitations...),
	}
	for _, f := range assessment.Findings {
		screen.Findings = append(screen.Findings, viewmodel.FindingRow{
			ID:         f.ID,
			Title:      f.Title,
			Severity:   f.Severity,
			Confidence: f.Confidence,
			Rationale:  f.Rationale,
		})
	}
	for _, step := range assessment.NextSteps {
		screen.NextSteps = append(screen.NextSteps, viewmodel.NextStepRow{
			ID:        step.ID,
			Title:     step.Title,
			Operation: step.Operation,
			Risk:      step.Risk,
			Rationale: step.Rationale,
		})
	}
	if sess != nil {
		screen.SessionID = sess.SessionID
	}
	model.Agent = screen
	if model.Overview.HasReport && screen.Headline != "" {
		model.Summary.Headline = screen.Headline
	}
}

func overviewFrom(report diagnostics.Report, mock bool) viewmodel.OverviewScreen {
	ok, warn, errn, unk := 0, 0, 0, 0
	for _, c := range report.Checks {
		switch strings.ToLower(c.Status) {
		case "ok":
			ok++
		case "warning":
			warn++
		case "error":
			errn++
		default:
			unk++
		}
	}
	runtime := strings.TrimSpace(report.Environment.RuntimeOS + "/" + report.Environment.RuntimeArch)
	screen := viewmodel.OverviewScreen{
		HasReport:     true,
		ReportID:      report.ReportID,
		SchemaVersion: report.SchemaVersion,
		CollectedAt:   formatTime(report.CollectedAt),
		FirmwareMode:  firstNonEmpty(report.Hardware.FirmwareMode, report.Boot.FirmwareMode),
		Manufacturer:  report.Hardware.System.Manufacturer,
		Model:         report.Hardware.System.Model,
		Processor:     report.Hardware.Processor.Name,
		MemoryBytes:   report.Hardware.Memory.TotalPhysicalBytes,
		Hostname:      report.Environment.Hostname,
		Runtime:       runtime,
		CheckOK:       ok,
		CheckWarning:  warn,
		CheckError:    errn,
		CheckUnknown:  unk,
		MockMode:      mock,
	}
	if mock {
		screen.StatusMessage = "msg.mock_mode"
	}
	return screen
}

func summaryFrom(report diagnostics.Report) viewmodel.SummaryScreen {
	screen := viewmodel.SummaryScreen{HasReport: true}
	for _, c := range report.Checks {
		screen.Checks = append(screen.Checks, viewmodel.CheckRow{
			ID:        c.ID,
			StatusKey: statusKey(c.Status),
			Summary:   c.Summary,
		})
	}
	return screen
}

func hardwareFrom(report diagnostics.Report) viewmodel.HardwareScreen {
	return viewmodel.HardwareScreen{
		HasReport:    true,
		FirmwareMode: firstNonEmpty(report.Hardware.FirmwareMode, report.Boot.FirmwareMode),
		Manufacturer: report.Hardware.System.Manufacturer,
		Model:        report.Hardware.System.Model,
		Processor:    report.Hardware.Processor.Name,
		Cores:        report.Hardware.Processor.Cores,
		LogicalCPUs:  report.Hardware.Processor.LogicalProcessors,
		MemoryBytes:  report.Hardware.Memory.TotalPhysicalBytes,
	}
}

func storageFrom(report diagnostics.Report) viewmodel.StorageScreen {
	screen := viewmodel.StorageScreen{HasReport: true}
	for _, d := range report.Storage.Disks {
		screen.Disks = append(screen.Disks, viewmodel.DiskRow{
			Number:            d.Number,
			Name:              d.FriendlyName,
			BusType:           d.BusType,
			SizeBytes:         d.SizeBytes,
			HealthStatus:      d.HealthStatus,
			OperationalStatus: d.OperationalStatus,
			IsBoot:            d.IsBoot,
			IsSystem:          d.IsSystem,
		})
	}
	for _, h := range report.Storage.DriveHealth {
		screen.Health = append(screen.Health, viewmodel.HealthRow{
			DeviceID:     h.DeviceID,
			Name:         h.FriendlyName,
			MediaType:    h.MediaType,
			HealthStatus: h.HealthStatus,
			Temperature:  optionalUint(h.TemperatureC),
			WearPercent:  optionalUint(h.WearPercent),
			PowerOnHours: optionalUint(h.PowerOnHours),
			ReadErrors:   optionalUint(h.ReadErrorsTotal),
			WriteErrors:  optionalUint(h.WriteErrorsTotal),
		})
	}
	for _, p := range report.Storage.Partitions {
		screen.Partitions = append(screen.Partitions, viewmodel.PartitionRow{
			DiskNumber:      p.DiskNumber,
			PartitionNumber: p.PartitionNumber,
			DriveLetter:     p.DriveLetter,
			SizeBytes:       p.SizeBytes,
			Type:            p.Type,
			IsActive:        p.IsActive,
		})
	}
	return screen
}

func bitlockerFrom(report diagnostics.Report) viewmodel.BitLockerScreen {
	status := strings.TrimSpace(report.Storage.BitLockerInventory.Status)
	screen := viewmodel.BitLockerScreen{
		HasReport:       true,
		InventoryStatus: status,
		InventoryError:  report.Storage.BitLockerInventory.Error,
	}
	switch status {
	case diagnostics.BitLockerStatusUnavailable:
		screen.VolumeCountKnown = false
		screen.StatusMessageKey = "msg.bitlocker_unavailable"
	case diagnostics.BitLockerStatusOK, diagnostics.BitLockerStatusPartial:
		screen.VolumeCountKnown = true
		if status == diagnostics.BitLockerStatusPartial {
			screen.StatusMessageKey = "status.partial"
		}
	default:
		screen.VolumeCountKnown = false
		screen.StatusMessageKey = "msg.data_source_unavailable"
	}
	if report.Storage.BitLockerVolumes == nil && status == diagnostics.BitLockerStatusUnavailable {
		screen.VolumeCountKnown = false
		if screen.StatusMessageKey == "" {
			screen.StatusMessageKey = "msg.volume_count_unknown"
		}
	}
	for _, v := range report.Storage.BitLockerVolumes {
		screen.Volumes = append(screen.Volumes, viewmodel.BitLockerRow{
			MountPoint:       v.MountPoint,
			VolumeStatus:     optionalStringPtr(v.VolumeStatus),
			ProtectionStatus: optionalStringPtr(v.ProtectionStatus),
			LockStatus:       optionalStringPtr(v.LockStatus),
			EncryptionMethod: optionalStringPtr(v.EncryptionMethod),
		})
	}
	return screen
}

func windowsFrom(report diagnostics.Report) viewmodel.WindowsScreen {
	screen := viewmodel.WindowsScreen{HasReport: true}
	if len(report.Installations) == 0 {
		screen.EmptyKey = "msg.no_windows_installs"
		return screen
	}
	for _, inst := range report.Installations {
		row := viewmodel.WindowsRow{Root: inst.Root}
		if inst.Version != nil {
			row.ProductName = firstNonEmpty(inst.Version.ProductName, inst.Version.RawProductName)
			row.DisplayVersion = inst.Version.DisplayVersion
			row.EditionID = inst.Version.EditionID
			row.Build = inst.Version.Build
			row.InstallType = inst.Version.InstallationType
		}
		screen.Installs = append(screen.Installs, row)
	}
	return screen
}

func networkFrom(report diagnostics.Report) viewmodel.NetworkScreen {
	screen := viewmodel.NetworkScreen{HasReport: true}
	ethernetUp := false
	for _, a := range report.Hardware.NetworkAdapters {
		norm := diagnostics.NormalizeNetworkAdapter(a)
		connected := norm.Status == diagnostics.NetStatusConnected ||
			norm.Status == diagnostics.NetStatusAuthenticationSucceeded
		if connected {
			ethernetUp = true
		}
		screen.Adapters = append(screen.Adapters, viewmodel.NetworkRow{
			Name:        norm.Name,
			Description: norm.Description,
			StatusKey:   networkStatusKey(norm.Status),
			StatusRaw:   norm.Status,
			Connected:   connected,
		})
	}
	screen.EthernetConnected = ethernetUp
	if !ethernetUp {
		screen.StatusMessageKey = "msg.ethernet_not_connected"
	}
	return screen
}

func optionalUint(v *uint64) viewmodel.OptionalString {
	if v == nil {
		return viewmodel.OptionalString{Available: false}
	}
	return viewmodel.OptionalString{Available: true, Value: strconv.FormatUint(*v, 10)}
}

func optionalStringPtr(v *string) viewmodel.OptionalString {
	if v == nil {
		return viewmodel.OptionalString{Available: false}
	}
	return viewmodel.OptionalString{Available: true, Value: *v}
}

func statusKey(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "ok":
		return "status.ok"
	case "warning":
		return "status.warning"
	case "error":
		return "status.error"
	default:
		return "status.unknown"
	}
}

func networkStatusKey(status string) string {
	switch status {
	case diagnostics.NetStatusConnected, diagnostics.NetStatusAuthenticationSucceeded:
		return "network.connected"
	case diagnostics.NetStatusDisconnected:
		return "network.disconnected"
	case diagnostics.NetStatusMediaDisconnected:
		return "network.media_disconnected"
	default:
		return "network.unknown"
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// FormatBytes renders a byte size for UI text panels.
func FormatBytes(v uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
		tb = gb * 1024
	)
	switch {
	case v >= tb:
		return fmt.Sprintf("%.1f TiB", float64(v)/float64(tb))
	case v >= gb:
		return fmt.Sprintf("%.1f GiB", float64(v)/float64(gb))
	case v >= mb:
		return fmt.Sprintf("%.1f MiB", float64(v)/float64(mb))
	case v >= kb:
		return fmt.Sprintf("%.1f KiB", float64(v)/float64(kb))
	default:
		return fmt.Sprintf("%d B", v)
	}
}

// DisplayOptional returns value or a localized n/a key marker.
func DisplayOptional(v viewmodel.OptionalString, na string) string {
	if !v.Available {
		return na
	}
	return v.Value
}
