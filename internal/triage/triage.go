package triage

import (
	"fmt"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

// Analyze creates an offline, deterministic diagnostic preflight. It does not
// execute repairs and does not claim that missing evidence proves a device is healthy.
func Analyze(report diagnostics.Report, now time.Time) (diagnosis.Assessment, error) {
	if report.ReportID == "" {
		return diagnosis.Assessment{}, fmt.Errorf("report_id is required")
	}
	if report.SchemaVersion != diagnostics.SchemaVersion {
		return diagnosis.Assessment{}, fmt.Errorf("unsupported diagnostic schema %q; expected %q", report.SchemaVersion, diagnostics.SchemaVersion)
	}

	assessment := diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      report.ReportID,
		GeneratedAt:   now.UTC(),
		Mode:          diagnosis.ModeOfflinePreflight,
		Findings:      []diagnosis.Finding{},
		Questions:     []diagnosis.Question{},
		NextSteps:     []diagnosis.NextStep{},
		Limitations: []string{
			"Offline preflight uses conservative deterministic rules; it is not the model-backed EffexorWinPE agent diagnosis.",
			"Missing providers or counters reduce confidence and never imply that hardware is healthy.",
			"This assessment contains read-only next steps only and cannot execute repair commands.",
		},
	}

	addSourceFindings(&assessment, report)
	addInstallationFindings(&assessment, report)
	addBootFindings(&assessment, report)
	addStorageFindings(&assessment, report)
	addBitLockerFindings(&assessment, report)

	if len(assessment.Findings) == 0 {
		assessment.Findings = append(assessment.Findings, diagnosis.Finding{
			ID:         "preflight.no-obvious-fault",
			Title:      "No obvious fault was identified in the collected evidence",
			Severity:   diagnosis.SeverityInfo,
			Confidence: diagnosis.ConfidenceLow,
			Rationale:  "The current report contains no rule-matched fault, but it does not include every diagnostic source and is not a clean bill of health.",
			EvidenceRefs: []string{
				"checks",
				"storage",
				"boot",
				"windows_installations",
			},
		})
	}

	assessment.Summary = summarize(assessment.Findings)
	return assessment, nil
}

func addSourceFindings(assessment *diagnosis.Assessment, report diagnostics.Report) {
	missing := 0
	for _, check := range report.Checks {
		if check.Status == "unknown" || check.Status == "error" {
			missing++
		}
	}
	if missing == 0 {
		return
	}
	assessment.Findings = append(assessment.Findings, diagnosis.Finding{
		ID:           "evidence.sources-incomplete",
		Title:        "Some diagnostic sources are unavailable",
		Severity:     diagnosis.SeverityUnknown,
		Confidence:   diagnosis.ConfidenceHigh,
		Rationale:    fmt.Sprintf("%d collector check(s) reported unknown or error status; conclusions that depend on those sources must stay provisional.", missing),
		EvidenceRefs: []string{"checks"},
	})
	addStep(assessment, diagnosis.NextStep{
		ID:        "review-missing-sources",
		Title:     "Review unavailable diagnostic providers",
		Operation: "review_missing_sources",
		Risk:      diagnosis.RiskReadOnly,
		Rationale: "Decide whether a storage driver, WinPE optional component, or vendor utility is required before diagnosis.",
	})
}

func addInstallationFindings(assessment *diagnosis.Assessment, report diagnostics.Report) {
	switch len(report.Installations) {
	case 0:
		assessment.Findings = append(assessment.Findings, diagnosis.Finding{
			ID:           "windows.installation-not-found",
			Title:        "No offline Windows installation was detected",
			Severity:     diagnosis.SeverityWarning,
			Confidence:   diagnosis.ConfidenceHigh,
			Rationale:    "Windows may be on an unmounted, encrypted, unsupported, or damaged volume, or the required storage driver may be missing.",
			EvidenceRefs: []string{"windows_installations", "storage.disks", "storage.bitlocker_volumes"},
		})
		addStep(assessment, diagnosis.NextStep{
			ID:        "identify-windows-volume",
			Title:     "Identify the Windows volume",
			Operation: "identify_windows_installation",
			Risk:      diagnosis.RiskReadOnly,
			Rationale: "Correlate visible disks, partitions, encryption state, and storage-driver availability.",
		})
		addQuestion(assessment, diagnosis.Question{
			ID:         "system-drive-visible-in-firmware",
			Prompt:     "Is the expected system drive visible in BIOS/UEFI setup or another operating system?",
			Reason:     "This separates a missing WinPE driver or locked volume from a drive that is not detected by the platform.",
			AnswerType: diagnosis.AnswerYesNo,
		})
	default:
		if len(report.Installations) > 1 {
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           "windows.multiple-installations",
				Title:        "Multiple Windows installations need target selection",
				Severity:     diagnosis.SeverityInfo,
				Confidence:   diagnosis.ConfidenceHigh,
				Rationale:    fmt.Sprintf("%d installations were detected; repair planning must identify the intended system before any offline operation.", len(report.Installations)),
				EvidenceRefs: []string{"windows_installations"},
			})
			addStep(assessment, diagnosis.NextStep{
				ID:        "select-windows-target",
				Title:     "Select the intended Windows installation",
				Operation: "select_windows_target",
				Risk:      diagnosis.RiskReadOnly,
				Rationale: "Prevent a later repair from targeting the wrong installation.",
			})
			addQuestion(assessment, diagnosis.Question{
				ID:         "intended-windows-installation",
				Prompt:     "Which detected Windows installation is the client's active system?",
				Reason:     "The agent must know the intended target before it can plan offline checks or repairs.",
				AnswerType: diagnosis.AnswerFreeText,
			})
		}
	}
}

func addBootFindings(assessment *diagnosis.Assessment, report diagnostics.Report) {
	if len(report.Boot.BCDStores) == 0 {
		assessment.Findings = append(assessment.Findings, diagnosis.Finding{
			ID:           "boot.bcd-not-visible",
			Title:        "No BCD store is visible on mounted volumes",
			Severity:     diagnosis.SeverityWarning,
			Confidence:   diagnosis.ConfidenceMedium,
			Rationale:    "The EFI system partition may be unmounted, the BCD may be missing, or the current scan may not see the relevant volume.",
			EvidenceRefs: []string{"boot.bcd_stores", "boot.firmware_mode"},
		})
		addStep(assessment, diagnosis.NextStep{
			ID:        "inspect-bcd",
			Title:     "Inspect boot configuration without modifying it",
			Operation: "inspect_bcd_entries",
			Risk:      diagnosis.RiskReadOnly,
			Rationale: "Enumerate the correct system partition and BCD entries before proposing boot repair.",
		})
		addQuestion(assessment, diagnosis.Question{
			ID:         "observed-boot-symptom",
			Prompt:     "What exactly happens during startup: no boot device, automatic repair loop, black screen, or a specific error?",
			Reason:     "The visible symptom changes which boot evidence should be collected next.",
			AnswerType: diagnosis.AnswerFreeText,
		})
		return
	}

	mode := strings.ToLower(report.Boot.FirmwareMode)
	if mode != "uefi" && mode != "bios" {
		return
	}
	matchingStore := false
	for _, store := range report.Boot.BCDStores {
		if strings.EqualFold(store.Kind, mode) {
			matchingStore = true
			break
		}
	}
	if !matchingStore {
		assessment.Findings = append(assessment.Findings, diagnosis.Finding{
			ID:           "boot.firmware-bcd-mismatch",
			Title:        "Visible BCD type does not match the current firmware mode",
			Severity:     diagnosis.SeverityWarning,
			Confidence:   diagnosis.ConfidenceMedium,
			Rationale:    fmt.Sprintf("The environment reports %s firmware, but no visible %s BCD store was found.", mode, mode),
			EvidenceRefs: []string{"boot.firmware_mode", "boot.bcd_stores"},
		})
		addStep(assessment, diagnosis.NextStep{
			ID:        "correlate-boot-targets",
			Title:     "Correlate firmware, system partition, and Windows loader",
			Operation: "inspect_bcd_entries",
			Risk:      diagnosis.RiskReadOnly,
			Rationale: "Confirm the actual boot path before any BCD or EFI write is proposed.",
		})
	}
}

func addStorageFindings(assessment *diagnosis.Assessment, report diagnostics.Report) {
	for index, disk := range report.Storage.Disks {
		status := strings.ToLower(strings.TrimSpace(disk.HealthStatus))
		if status != "" && status != "healthy" && status != "ok" {
			severity := diagnosis.SeverityWarning
			if strings.Contains(status, "unhealthy") || strings.Contains(status, "failed") {
				severity = diagnosis.SeverityCritical
			}
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           fmt.Sprintf("storage.disk.%d.health", disk.Number),
				Title:        fmt.Sprintf("Disk %d reports %s health", disk.Number, disk.HealthStatus),
				Severity:     severity,
				Confidence:   diagnosis.ConfidenceHigh,
				Rationale:    "The operating system storage provider is reporting a non-healthy state; preserve data before repair attempts and verify with device-specific diagnostics.",
				EvidenceRefs: []string{fmt.Sprintf("storage.disks[%d].health_status", index)},
			})
			addStorageStep(assessment)
		}
	}

	for index, health := range report.Storage.DriveHealth {
		status := strings.ToLower(strings.TrimSpace(health.HealthStatus))
		if status != "" && status != "healthy" && status != "ok" {
			severity := diagnosis.SeverityWarning
			if strings.Contains(status, "unhealthy") || strings.Contains(status, "failed") {
				severity = diagnosis.SeverityCritical
			}
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           "storage.drive." + safeID(health.DeviceID) + ".health",
				Title:        fmt.Sprintf("Drive %s reports %s health", displayDevice(health), health.HealthStatus),
				Severity:     severity,
				Confidence:   diagnosis.ConfidenceHigh,
				Rationale:    "The physical-disk provider is reporting a non-healthy state; this should be verified before writes or filesystem repair.",
				EvidenceRefs: []string{fmt.Sprintf("storage.drive_health[%d].health_status", index)},
			})
			addStorageStep(assessment)
		}
		if health.TemperatureC != nil && *health.TemperatureC >= 70 {
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           "storage.drive." + safeID(health.DeviceID) + ".temperature",
				Title:        fmt.Sprintf("Drive %s reports a high temperature reading", displayDevice(health)),
				Severity:     diagnosis.SeverityWarning,
				Confidence:   diagnosis.ConfidenceMedium,
				Rationale:    fmt.Sprintf("The reported temperature is %d C. Controller reporting and vendor limits vary, so confirm the value and improve cooling before stress tests.", *health.TemperatureC),
				EvidenceRefs: []string{fmt.Sprintf("storage.drive_health[%d].temperature_celsius", index)},
			})
			addStorageStep(assessment)
		}
		if health.WearPercent != nil && *health.WearPercent >= 80 {
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           "storage.drive." + safeID(health.DeviceID) + ".wear",
				Title:        fmt.Sprintf("Drive %s reports high wear", displayDevice(health)),
				Severity:     diagnosis.SeverityWarning,
				Confidence:   diagnosis.ConfidenceMedium,
				Rationale:    fmt.Sprintf("The storage provider reports %d%% wear. Interpret this with the device model and vendor utility before declaring failure.", *health.WearPercent),
				EvidenceRefs: []string{fmt.Sprintf("storage.drive_health[%d].wear_percent", index)},
			})
			addStorageStep(assessment)
		}
		if positive(health.ReadErrorsTotal) || positive(health.WriteErrorsTotal) {
			assessment.Findings = append(assessment.Findings, diagnosis.Finding{
				ID:           "storage.drive." + safeID(health.DeviceID) + ".errors",
				Title:        fmt.Sprintf("Drive %s reports I/O error counters", displayDevice(health)),
				Severity:     diagnosis.SeverityWarning,
				Confidence:   diagnosis.ConfidenceMedium,
				Rationale:    "Non-zero counters can include corrected or historical events; preserve data and correlate them with vendor SMART/NVMe data and logs.",
				EvidenceRefs: []string{fmt.Sprintf("storage.drive_health[%d].read_errors_total", index), fmt.Sprintf("storage.drive_health[%d].write_errors_total", index)},
			})
			addStorageStep(assessment)
		}
	}
}

func addBitLockerFindings(assessment *diagnosis.Assessment, report diagnostics.Report) {
	locked := 0
	for _, volume := range report.Storage.BitLockerVolumes {
		if strings.EqualFold(volume.LockStatus, "locked") {
			locked++
		}
	}
	if locked == 0 {
		return
	}
	assessment.Findings = append(assessment.Findings, diagnosis.Finding{
		ID:           "bitlocker.locked-volumes",
		Title:        "One or more BitLocker volumes are locked",
		Severity:     diagnosis.SeverityInfo,
		Confidence:   diagnosis.ConfidenceHigh,
		Rationale:    fmt.Sprintf("%d volume(s) are locked. This can explain missing Windows files, but encryption itself is not a fault.", locked),
		EvidenceRefs: []string{"storage.bitlocker_volumes"},
	})
	addStep(assessment, diagnosis.NextStep{
		ID:        "review-bitlocker-access",
		Title:     "Review authorized BitLocker access",
		Operation: "review_bitlocker_access",
		Risk:      diagnosis.RiskReadOnly,
		Rationale: "Ask the device owner for an authorized recovery method; never collect or upload recovery keys in the diagnostic report.",
	})
	addQuestion(assessment, diagnosis.Question{
		ID:         "authorized-bitlocker-access",
		Prompt:     "Has the device owner provided an authorized BitLocker recovery method?",
		Reason:     "Offline Windows evidence may remain unavailable until the owner authorizes access.",
		AnswerType: diagnosis.AnswerYesNo,
	})
}

func addStorageStep(assessment *diagnosis.Assessment) {
	addStep(assessment, diagnosis.NextStep{
		ID:        "verify-storage-health",
		Title:     "Verify storage health and prioritize data preservation",
		Operation: "inspect_storage_health",
		Risk:      diagnosis.RiskReadOnly,
		Rationale: "Collect vendor-specific SMART/NVMe evidence before CHKDSK, cloning decisions, or other writes.",
	})
	addQuestion(assessment, diagnosis.Question{
		ID:         "client-data-backed-up",
		Prompt:     "Is the client's important data already backed up or cloned?",
		Reason:     "Possible storage degradation changes the priority from repair attempts to data preservation.",
		AnswerType: diagnosis.AnswerYesNo,
	})
}

func addQuestion(assessment *diagnosis.Assessment, question diagnosis.Question) {
	for _, existing := range assessment.Questions {
		if existing.ID == question.ID {
			return
		}
	}
	assessment.Questions = append(assessment.Questions, question)
}

func addStep(assessment *diagnosis.Assessment, step diagnosis.NextStep) {
	for _, existing := range assessment.NextSteps {
		if existing.ID == step.ID {
			return
		}
	}
	step.RequiresConfirmation = step.Risk != diagnosis.RiskReadOnly
	assessment.NextSteps = append(assessment.NextSteps, step)
}

func positive(value *uint64) bool {
	return value != nil && *value > 0
}

func safeID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var result strings.Builder
	for _, char := range strings.ToLower(value) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' {
			result.WriteRune(char)
		} else {
			result.WriteByte('-')
		}
	}
	normalized := strings.Trim(result.String(), "-")
	if normalized == "" {
		return "unknown"
	}
	return normalized
}

func displayDevice(health diagnostics.DriveHealth) string {
	if health.FriendlyName != "" {
		return health.FriendlyName
	}
	if health.DeviceID != "" {
		return health.DeviceID
	}
	return "unknown"
}

func summarize(findings []diagnosis.Finding) diagnosis.Summary {
	highest := diagnosis.SeverityInfo
	for _, finding := range findings {
		if severityRank(finding.Severity) > severityRank(highest) {
			highest = finding.Severity
		}
	}
	headline := "Preflight found no obvious fault in the available evidence"
	switch highest {
	case diagnosis.SeverityCritical:
		headline = "Preflight found evidence that requires immediate technician attention"
	case diagnosis.SeverityWarning:
		headline = "Preflight found issues that require additional diagnosis"
	case diagnosis.SeverityUnknown:
		headline = "Preflight is limited by missing diagnostic evidence"
	}
	return diagnosis.Summary{Headline: headline, HighestSeverity: highest, FindingCount: len(findings)}
}

func severityRank(value string) int {
	switch value {
	case diagnosis.SeverityCritical:
		return 4
	case diagnosis.SeverityWarning:
		return 3
	case diagnosis.SeverityUnknown:
		return 2
	default:
		return 1
	}
}
