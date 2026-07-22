package triage

import (
	"encoding/json"
	"os"
	"path/filepath"
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

func TestAnalyzePhysicalSmokeClassDoesNotFlagRuntimeWinPEAsSecondInstall(t *testing.T) {
	report := loadPhysicalSmokeFixture(t)
	assessment, err := Analyze(report, time.Unix(1700000000, 0).UTC())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if hasFinding(assessment, "windows.multiple-installations") {
		t.Fatal("runtime WinPE must not create windows.multiple-installations")
	}
	if !hasFinding(assessment, "evidence.sources-incomplete") {
		t.Fatal("unavailable BitLocker provider must keep evidence.sources-incomplete")
	}
	if assessment.Summary.HighestSeverity != diagnosis.SeverityUnknown {
		t.Fatalf("highest severity = %q, want unknown when sources are incomplete", assessment.Summary.HighestSeverity)
	}
	for _, step := range assessment.NextSteps {
		if step.Risk != diagnosis.RiskReadOnly || step.RequiresConfirmation {
			t.Fatalf("offline next step is not read-only: %+v", step)
		}
	}
}

func TestAnalyzeMultipleRealInstallationsStillFlagsTargetSelection(t *testing.T) {
	report := baseReport()
	report.Installations = []diagnostics.Installation{
		{Root: "C:\\"},
		{Root: "D:\\"},
	}
	assessment, err := Analyze(report, time.Now())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if !hasFinding(assessment, "windows.multiple-installations") {
		t.Fatal("missing multiple-installations finding for two real offline installs")
	}
}

func TestAnalyzeDoesNotTreatNullWearAsHealthyProof(t *testing.T) {
	report := baseReport()
	report.Storage.DriveHealth = []diagnostics.DriveHealth{{
		DeviceID:     "usb0",
		FriendlyName: "USB Flash",
		HealthStatus: "Healthy",
	}}
	assessment, err := Analyze(report, time.Now())
	if err != nil {
		t.Fatalf("Analyze() error = %v", err)
	}
	if hasFinding(assessment, "storage.drive.usb0.wear") {
		t.Fatal("null wear must not create a wear finding")
	}
	if !hasFinding(assessment, "preflight.no-obvious-fault") {
		t.Fatal("missing metrics must not invent a stronger healthy claim than the conservative default")
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
			BitLockerInventory: diagnostics.BitLockerInventory{
				Status: diagnostics.BitLockerStatusOK,
			},
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

func loadPhysicalSmokeFixture(t *testing.T) diagnostics.Report {
	t.Helper()
	path := filepath.Join("testdata", "physical-smoke-uefi-asus-like.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var report diagnostics.Report
	if err := json.Unmarshal(raw, &report); err != nil {
		t.Fatalf("decode fixture: %v", err)
	}
	if len(report.Installations) != 1 || report.Installations[0].Root != `D:\` {
		t.Fatalf("fixture must model one offline Windows root on D:\\, got %+v", report.Installations)
	}
	if report.Storage.BitLockerVolumes != nil || report.Storage.BitLockerInventory.Status != diagnostics.BitLockerStatusUnavailable {
		t.Fatal("fixture must model unavailable BitLocker as null volumes + unavailable status")
	}
	return report
}
