package triage

import (
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestAnalyzeRejectsUnsupportedReport(t *testing.T) {
	_, err := Analyze(diagnostics.Report{SchemaVersion: "0.0.0", ReportID: "report-1"}, time.Now())
	if err == nil {
		t.Fatal("Analyze() error = nil, want unsupported schema error")
	}
}

func TestAnalyzeFlagsUnhealthyStorageWithoutWriteActions(t *testing.T) {
	report := baseReport()
	report.Storage.Disks = []diagnostics.Disk{{Number: 0, HealthStatus: "Unhealthy"}}
	report.Storage.DriveHealth = []diagnostics.DriveHealth{{DeviceID: "0", FriendlyName: "Test SSD", HealthStatus: "Warning"}}

	assessment, err := Analyze(report, time.Unix(100, 0))
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if assessment.Summary.HighestSeverity != diagnosis.SeverityCritical {
		t.Fatalf("highest severity = %q, want critical", assessment.Summary.HighestSeverity)
	}
	if !hasFinding(assessment, "storage.disk.0.health") {
		t.Fatal("missing unhealthy disk finding")
	}
	if !hasQuestion(assessment, "client-data-backed-up") {
		t.Fatal("missing data-preservation question")
	}
	for _, step := range assessment.NextSteps {
		if step.Risk != diagnosis.RiskReadOnly || step.RequiresConfirmation {
			t.Fatalf("offline next step is not read-only: %+v", step)
		}
	}
}

func TestAnalyzeDoesNotCallLockedBitLockerAFault(t *testing.T) {
	report := baseReport()
	report.Storage.BitLockerVolumes = []diagnostics.BitLockerVolume{{MountPoint: "C:", LockStatus: "Locked"}}

	assessment, err := Analyze(report, time.Now())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !hasFinding(assessment, "bitlocker.locked-volumes") {
		t.Fatal("missing BitLocker access finding")
	}
	for _, finding := range assessment.Findings {
		if finding.ID == "bitlocker.locked-volumes" && finding.Severity != diagnosis.SeverityInfo {
			t.Fatalf("BitLocker severity = %q, want info", finding.Severity)
		}
	}
}

func TestAnalyzeFlagsFirmwareBCDMismatch(t *testing.T) {
	report := baseReport()
	report.Boot = diagnostics.Boot{FirmwareMode: "uefi", BCDStores: []diagnostics.BCDStore{{Path: "C:\\Boot\\BCD", Kind: "bios"}}}

	assessment, err := Analyze(report, time.Now())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !hasFinding(assessment, "boot.firmware-bcd-mismatch") {
		t.Fatal("missing firmware/BCD mismatch finding")
	}
}

func TestAnalyzeHealthyEvidenceStaysLowConfidence(t *testing.T) {
	report := baseReport()
	assessment, err := Analyze(report, time.Now())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !hasFinding(assessment, "preflight.no-obvious-fault") {
		t.Fatal("missing conservative no-obvious-fault finding")
	}
	if assessment.Findings[0].Confidence != diagnosis.ConfidenceLow {
		t.Fatalf("confidence = %q, want low", assessment.Findings[0].Confidence)
	}
}

func baseReport() diagnostics.Report {
	return diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      "report-1234567890",
		Hardware:      diagnostics.Hardware{FirmwareMode: "uefi", NetworkAdapters: []diagnostics.NetworkAdapter{}},
		Storage: diagnostics.Storage{
			Disks:            []diagnostics.Disk{{Number: 0, HealthStatus: "Healthy"}},
			DriveHealth:      []diagnostics.DriveHealth{},
			Partitions:       []diagnostics.Partition{},
			BitLockerVolumes: []diagnostics.BitLockerVolume{},
		},
		Boot:          diagnostics.Boot{FirmwareMode: "uefi", BCDStores: []diagnostics.BCDStore{{Path: "S:\\EFI\\Microsoft\\Boot\\BCD", Kind: "uefi"}}},
		Installations: []diagnostics.Installation{{Root: "C:\\"}},
		Checks:        []diagnostics.Check{{ID: "platform.inventory", Status: "ok", Summary: "available"}},
	}
}

func hasFinding(assessment diagnosis.Assessment, id string) bool {
	for _, finding := range assessment.Findings {
		if finding.ID == id {
			return true
		}
	}
	return false
}

func hasQuestion(assessment diagnosis.Assessment, id string) bool {
	for _, question := range assessment.Questions {
		if question.ID == id {
			return true
		}
	}
	return false
}
