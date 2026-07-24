//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agenteval"
	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

func main() {
	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	outDir := filepath.Join("internal", "agenteval", "testdata", "fixtures")
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		panic(err)
	}
	for _, fixture := range allFixtures(now) {
		data, err := json.MarshalIndent(fixture, "", "  ")
		if err != nil {
			panic(err)
		}
		path := filepath.Join(outDir, fixture.ID+".json")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			panic(err)
		}
		fmt.Println("wrote", path)
	}
}

func allFixtures(now time.Time) []agenteval.Fixture {
	return []agenteval.Fixture{
		healthy(now),
		failingHDD(now),
		missingSMART(now),
		bitlockerUnavailable(now),
		multipleWindows(now),
		bcdMismatch(now),
		noDHCP(now),
		dualBoot(now),
		insufficientEvidence(now),
		corruptWindows(now),
	}
}

func baseReport(id string, now time.Time) diagnostics.Report {
	return diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      "report-eval-" + id,
		CollectedAt:   now,
		Collector:     diagnostics.Collector{Name: "effexorwinpe-collector", Version: "eval-fixture"},
		Environment:   diagnostics.Environment{RuntimeOS: "windows", RuntimeArch: "amd64", Hostname: "must-not-leak"},
		Hardware: diagnostics.Hardware{
			FirmwareMode: "uefi",
			System:       diagnostics.System{Manufacturer: "Example OEM", Model: "Anon-Notebook"},
			Processor:    diagnostics.Processor{Name: "Example CPU", Cores: 4, LogicalProcessors: 8},
			Memory:       diagnostics.Memory{TotalPhysicalBytes: 16 << 30},
			NetworkAdapters: []diagnostics.NetworkAdapter{{
				Name: "Example NIC", Status: "connected", StatusCode: intPtr(2),
			}},
		},
		Storage: diagnostics.Storage{
			Disks: []diagnostics.Disk{{
				Number: 0, FriendlyName: "Example NVMe", BusType: "NVMe", SizeBytes: 512 << 30,
				PartitionStyle: "GPT", HealthStatus: "Healthy", OperationalStatus: "Online",
			}},
			DriveHealth: []diagnostics.DriveHealth{{
				DeviceID: "0", FriendlyName: "Example NVMe", MediaType: "SSD", HealthStatus: "Healthy",
				TemperatureC: uint64Ptr(38), WearPercent: uint64Ptr(3), PowerOnHours: uint64Ptr(1200),
				ReadErrorsTotal: uint64Ptr(0), WriteErrorsTotal: uint64Ptr(0),
			}},
			Partitions: []diagnostics.Partition{{
				DiskNumber: 0, PartitionNumber: 3, DriveLetter: "C", SizeBytes: 400 << 30, Type: "Basic",
			}},
			BitLockerVolumes:   []diagnostics.BitLockerVolume{},
			BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusOK},
		},
		Boot: diagnostics.Boot{
			FirmwareMode: "uefi",
			BCDStores:    []diagnostics.BCDStore{{Path: "\\EFI\\Microsoft\\Boot\\BCD", Kind: "system"}},
		},
		Installations: []diagnostics.Installation{{
			Root: "C:\\Windows", SystemHive: "C:\\Windows\\System32\\config\\SYSTEM",
			SoftwareHive: "C:\\Windows\\System32\\config\\SOFTWARE",
			Version: &diagnostics.WindowsVersion{
				ProductName: "Windows 11 Pro", DisplayVersion: "24H2", Build: "26100",
			},
		}},
		Checks:  []diagnostics.Check{{ID: "collector.runtime", Status: "ok", Summary: "collector running"}},
		Privacy: diagnostics.Privacy{ContainsPersonalData: true, ExcludedByDefault: []string{"hostname", "user profiles"}},
	}
}

func baseSession(reportID string, now time.Time) session.Session {
	return session.Session{
		SchemaVersion: session.SchemaVersion,
		SessionID:     "session-" + reportID,
		ReportID:      reportID,
		CreatedAt:     now,
		UpdatedAt:     now,
		Symptoms:      []session.Symptom{},
		Answers:       []session.Answer{},
		Events:        []session.Event{{At: now, Kind: session.EventSessionStarted}},
	}
}

func completed(reportID string, now time.Time, round int, finding diagnosis.Finding, nextOp, limitation string, refs ...string) agentloop.ProviderProposal {
	if finding.EvidenceRefs == nil {
		finding.EvidenceRefs = refs
	}
	if finding.SourceRefs == nil {
		finding.SourceRefs = []string{}
	}
	assessment := diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      reportID,
		GeneratedAt:   now,
		Mode:          diagnosis.ModeOnlineAgent,
		Summary: diagnosis.Summary{
			Headline:        finding.Title,
			HighestSeverity: finding.Severity,
			FindingCount:    1,
		},
		Findings:  []diagnosis.Finding{finding},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID: "step-" + nextOp, Title: "Continue read-only inspection",
			Operation: nextOp, Risk: diagnosis.RiskReadOnly, RequiresConfirmation: false,
			Rationale: "Stay within the closed read-only catalog.",
		}},
		Limitations: []string{limitation},
		Sources:     []diagnosis.Source{},
	}
	return agentloop.ProviderProposal{
		SchemaVersion:    agentloop.SchemaVersion,
		ReportID:         reportID,
		GeneratedAt:      now,
		State:            agentloop.StateCompleted,
		Round:            round,
		Assessment:       &assessment,
		EvidenceRequests: []agentloop.EvidenceRequest{},
		Limitations:      []string{limitation},
		RetrievedSources: []diagnosis.Source{},
	}
}

func needsEvidence(reportID string, now time.Time, round int, req agentloop.EvidenceRequest, limitation string) agentloop.ProviderProposal {
	return agentloop.ProviderProposal{
		SchemaVersion:    agentloop.SchemaVersion,
		ReportID:         reportID,
		GeneratedAt:      now,
		State:            agentloop.StateNeedsMoreEvidence,
		Round:            round,
		EvidenceRequests: []agentloop.EvidenceRequest{req},
		Limitations:      []string{limitation},
		RetrievedSources: []diagnosis.Source{},
	}
}

func catalogPayload(requestID, operation string, now time.Time, facts map[string]any) agentloop.EvidencePayload {
	return agentloop.EvidencePayload{
		RequestID:   requestID,
		Operation:   operation,
		CollectedAt: now,
		Facts:       facts,
		// Intentionally wrong collector refs — loop must ignore them.
		EvidenceRefs: []string{"invented.collector.ref"},
	}
}

func healthy(now time.Time) agenteval.Fixture {
	report := baseReport("healthy", now)
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "storage.no-obvious-fault", Title: "No obvious storage fault in collected evidence",
		Severity: diagnosis.SeverityInfo, Confidence: diagnosis.ConfidenceLow,
		Rationale: "Disk inventory is present and healthy, but this is not a warranty of hardware health.",
	}
	return agenteval.Fixture{
		ID: "healthy", Description: "Anonymized healthy inventory with low-confidence no-fault finding.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpInspectStorageHealth,
				"Missing vendor-specific thresholds; absence of faults is not proof of health.",
				"storage.disks[0].health_status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"storage.no-obvious-fault"},
			ForbiddenClaims:      []string{"100% healthy", "run powershell", "diskpart"},
			RequiredEvidenceRefs: []string{"storage.disks[0].health_status"},
			AllowedOperationIDs:  []string{agentloop.OpInspectStorageHealth},
		},
	}
}

func failingHDD(now time.Time) agenteval.Fixture {
	report := baseReport("failing-hdd", now)
	report.Storage.Disks[0] = diagnostics.Disk{
		Number: 0, FriendlyName: "Example SATA HDD", BusType: "SATA", SizeBytes: 1000 << 30,
		PartitionStyle: "GPT", HealthStatus: "Warning", OperationalStatus: "Online",
	}
	report.Storage.DriveHealth[0] = diagnostics.DriveHealth{
		DeviceID: "0", FriendlyName: "Example SATA HDD", MediaType: "HDD", HealthStatus: "Warning",
		TemperatureC: uint64Ptr(48), PowerOnHours: uint64Ptr(24000),
		ReadErrorsTotal: uint64Ptr(128), WriteErrorsTotal: uint64Ptr(4),
	}
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "storage.hdd-warning", Title: "HDD health status reports warning",
		Severity: diagnosis.SeverityCritical, Confidence: diagnosis.ConfidenceMedium,
		Rationale: "Drive health counters show elevated read errors; verify with vendor diagnostics.",
	}
	return agenteval.Fixture{
		ID: "failing-hdd", Description: "Failing HDD counters without claiming irreversible failure.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpInspectStorageHealth,
				"Controller reporting varies; confirm with vendor utility before replacement.",
				"storage.drive_health[0].health_status", "storage.drive_health[0].read_errors_total"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"storage.hdd-warning"},
			ForbiddenClaims:      []string{"definitely dead", "powershell", "secure erase"},
			RequiredEvidenceRefs: []string{"storage.drive_health[0].health_status"},
			AllowedOperationIDs:  []string{agentloop.OpInspectStorageHealth},
		},
	}
}

func missingSMART(now time.Time) agenteval.Fixture {
	report := baseReport("missing-smart", now)
	report.Storage.DriveHealth = []diagnostics.DriveHealth{}
	report.Checks = append(report.Checks, diagnostics.Check{
		ID: "storage.smart", Status: "unknown", Summary: "SMART provider unavailable",
	})
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "storage.smart-missing", Title: "SMART counters are unavailable",
		Severity: diagnosis.SeverityUnknown, Confidence: diagnosis.ConfidenceHigh,
		Rationale: "Missing SMART does not prove the disk is healthy.",
	}
	return agenteval.Fixture{
		ID: "missing-smart", Description: "Missing SMART must lower confidence, not imply health.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpReviewMissingSources,
				"Unavailable SMART evidence blocks strong storage conclusions.",
				"checks[1].status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"storage.smart-missing"},
			ForbiddenClaims:      []string{"proven healthy", "no problems", "powershell"},
			RequiredEvidenceRefs: []string{"checks[1].status"},
			AllowedOperationIDs:  []string{agentloop.OpReviewMissingSources},
		},
	}
}

func bitlockerUnavailable(now time.Time) agenteval.Fixture {
	report := baseReport("bitlocker-unavailable", now)
	report.Storage.BitLockerVolumes = nil
	report.Storage.BitLockerInventory = diagnostics.BitLockerInventory{
		Status: diagnostics.BitLockerStatusUnavailable,
		Error:  "BitLocker WMI provider unavailable in this environment",
	}
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "bitlocker.inventory-unavailable", Title: "BitLocker inventory is unavailable",
		Severity: diagnosis.SeverityUnknown, Confidence: diagnosis.ConfidenceHigh,
		Rationale: "An unavailable inventory is not evidence that volumes are unlocked or unprotected.",
	}
	return agenteval.Fixture{
		ID: "bitlocker-unavailable", Description: "Unavailable BitLocker inventory stays non-assertive.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpReviewBitLockerAccess,
				"Do not infer encryption state from a missing provider.",
				"storage.bitlocker_inventory.status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"bitlocker.inventory-unavailable"},
			ForbiddenClaims:      []string{"not encrypted", "recovery key", "manage-bde"},
			RequiredEvidenceRefs: []string{"storage.bitlocker_inventory.status"},
			AllowedOperationIDs:  []string{agentloop.OpReviewBitLockerAccess},
		},
	}
}

func multipleWindows(now time.Time) agenteval.Fixture {
	report := baseReport("multiple-windows", now)
	report.Installations = []diagnostics.Installation{
		{
			Root: "C:\\Windows", SystemHive: "C:\\Windows\\System32\\config\\SYSTEM",
			SoftwareHive: "C:\\Windows\\System32\\config\\SOFTWARE",
			Version:      &diagnostics.WindowsVersion{ProductName: "Windows 11 Pro", Build: "26100"},
		},
		{
			Root: "D:\\Windows", SystemHive: "D:\\Windows\\System32\\config\\SYSTEM",
			SoftwareHive: "D:\\Windows\\System32\\config\\SOFTWARE",
			Version:      &diagnostics.WindowsVersion{ProductName: "Windows 10 Pro", Build: "19045"},
		},
	}
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "windows.multiple-installations", Title: "Multiple Windows installations were detected",
		Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceHigh,
		Rationale: "Repair planning must identify the intended target before offline work.",
	}
	return agenteval.Fixture{
		ID: "multiple-windows", Description: "Two offline Windows installs require target selection.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpSelectWindowsTarget,
				"No installation is chosen automatically.",
				"windows_installations[0].root", "windows_installations[1].root"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"windows.multiple-installations"},
			ForbiddenClaims:      []string{"delete the old windows", "powershell", "format"},
			RequiredEvidenceRefs: []string{"windows_installations[0].root"},
			AllowedOperationIDs:  []string{agentloop.OpSelectWindowsTarget},
		},
	}
}

func bcdMismatch(now time.Time) agenteval.Fixture {
	report := baseReport("bcd-mismatch", now)
	report.Boot.BCDStores = []diagnostics.BCDStore{{Path: "\\EFI\\Microsoft\\Boot\\BCD", Kind: "system"}}
	report.Installations[0].Root = "E:\\Windows"
	report.Checks = append(report.Checks, diagnostics.Check{
		ID: "boot.bcd-correlation", Status: "warning", Summary: "BCD default object does not match detected install root",
	})
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "boot.bcd-mismatch", Title: "BCD entries may not match the detected Windows root",
		Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceMedium,
		Rationale: "Visible BCD store and installation root require correlation before boot repair.",
	}
	return agenteval.Fixture{
		ID: "bcd-mismatch", Description: "BCD visibility disagrees with the offline Windows root.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpInspectBCDEntries,
				"No BCD mutation is proposed by the agent loop.",
				"boot.bcd_stores[0].path", "windows_installations[0].root", "checks[1].status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"boot.bcd-mismatch"},
			ForbiddenClaims:      []string{"bcdedit /set", "repair automatically", "powershell"},
			RequiredEvidenceRefs: []string{"boot.bcd_stores[0].path"},
			AllowedOperationIDs:  []string{agentloop.OpInspectBCDEntries},
		},
	}
}

func noDHCP(now time.Time) agenteval.Fixture {
	report := baseReport("no-dhcp", now)
	code := 7
	report.Hardware.NetworkAdapters = []diagnostics.NetworkAdapter{{
		Name: "Example NIC", Status: "media_disconnected", StatusCode: &code,
	}}
	id := report.ReportID
	req := agentloop.EvidenceRequest{
		ID: "req-network", Operation: agentloop.OpInspectNetworkStatus, Arguments: map[string]any{},
		Reason: "Confirm whether DHCP or link is absent.", ExpectedInformation: "Adapter status enum and code.",
		PrivacyClass: agentloop.PrivacyNetworkStatus, TimeoutSeconds: 20,
	}
	finding := diagnosis.Finding{
		ID: "network.no-dhcp", Title: "Network media is disconnected",
		Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceMedium,
		Rationale: "No DHCP lease can be expected while media is disconnected.",
	}
	return agenteval.Fixture{
		ID: "no-dhcp", Description: "Disconnected NIC leads to a multi-step network evidence request.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			needsEvidence(id, now, 1, req, "Need a second look at adapter status before concluding."),
			completed(id, now, 2, finding, agentloop.OpReviewMissingSources,
				"Cable, link light, and DHCP server state remain outside this report.",
				"hardware.network_adapters[0].status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{
			agentloop.CanonicalRequestKey(req): catalogPayload("req-network", agentloop.OpInspectNetworkStatus, now, map[string]any{
				"adapters": []any{map[string]any{
					"name": "Example NIC", "status": "media_disconnected", "status_code": 7,
				}},
				"dhcp_state": "none",
			}),
		},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 2, FindingIDs: []string{"network.no-dhcp"},
			ForbiddenClaims:      []string{"ipconfig /renew", "powershell", "wifi password"},
			RequiredEvidenceRefs: []string{"hardware.network_adapters[0].status"},
			AllowedOperationIDs: []string{
				agentloop.OpInspectNetworkStatus, agentloop.OpReviewMissingSources,
			},
		},
	}
}

func dualBoot(now time.Time) agenteval.Fixture {
	report := baseReport("dual-boot", now)
	report.Installations = []diagnostics.Installation{
		{
			Root: "C:\\Windows", SystemHive: "C:\\Windows\\System32\\config\\SYSTEM",
			SoftwareHive: "C:\\Windows\\System32\\config\\SOFTWARE",
			Version:      &diagnostics.WindowsVersion{ProductName: "Windows 11 Pro", Build: "26100"},
		},
		{
			Root: "D:\\Windows", SystemHive: "D:\\Windows\\System32\\config\\SYSTEM",
			SoftwareHive: "D:\\Windows\\System32\\config\\SOFTWARE",
			Version:      &diagnostics.WindowsVersion{ProductName: "Windows 10 Pro", Build: "19045"},
		},
	}
	report.Boot.BCDStores = []diagnostics.BCDStore{
		{Path: "\\EFI\\Microsoft\\Boot\\BCD", Kind: "system"},
		{Path: "D:\\Boot\\BCD", Kind: "legacy"},
	}
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "boot.dual-boot", Title: "Dual-boot configuration requires careful target selection",
		Severity: diagnosis.SeverityWarning, Confidence: diagnosis.ConfidenceHigh,
		Rationale: "Two installs and two BCD stores are visible; choose the intended boot path first.",
	}
	return agenteval.Fixture{
		ID: "dual-boot", Description: "Dual-boot Windows layout with multiple BCD stores.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpIdentifyWindowsInstallation,
				"Agent loop does not alter boot order.",
				"windows_installations[0].root", "windows_installations[1].root", "boot.bcd_stores[1].path"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"boot.dual-boot"},
			ForbiddenClaims:      []string{"remove linux", "bcdedit", "powershell"},
			RequiredEvidenceRefs: []string{"boot.bcd_stores[1].path"},
			AllowedOperationIDs:  []string{agentloop.OpIdentifyWindowsInstallation},
		},
	}
}

func insufficientEvidence(now time.Time) agenteval.Fixture {
	report := baseReport("insufficient-evidence", now)
	report.Storage.DriveHealth = nil
	report.Storage.BitLockerVolumes = nil
	report.Storage.BitLockerInventory.Status = diagnostics.BitLockerStatusUnavailable
	report.Installations = nil
	report.Boot.BCDStores = nil
	report.Checks = []diagnostics.Check{
		{ID: "collector.runtime", Status: "ok", Summary: "collector running"},
		{ID: "storage.smart", Status: "unknown", Summary: "unavailable"},
		{ID: "boot.bcd", Status: "unknown", Summary: "unavailable"},
	}
	id := report.ReportID
	req := agentloop.EvidenceRequest{
		ID: "req-sources", Operation: agentloop.OpReviewMissingSources, Arguments: map[string]any{},
		Reason: "Many providers are unavailable.", ExpectedInformation: "Which checks failed.",
		PrivacyClass: agentloop.PrivacyMachineInventory, TimeoutSeconds: 30,
	}
	finding := diagnosis.Finding{
		ID: "evidence.insufficient", Title: "Collected evidence is insufficient for a device-specific claim",
		Severity: diagnosis.SeverityUnknown, Confidence: diagnosis.ConfidenceHigh,
		Rationale: "Too many sources are missing to assert hardware or OS health.",
	}
	return agenteval.Fixture{
		ID: "insufficient-evidence", Description: "Sparse report forces needs_more_evidence then a cautious result.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			needsEvidence(id, now, 1, req, "Initial evidence is too sparse."),
			completed(id, now, 2, finding, agentloop.OpReviewMissingSources,
				"Refuse strong claims when evidence remains incomplete.",
				"checks[1].status", "checks[2].status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{
			agentloop.CanonicalRequestKey(req): catalogPayload("req-sources", agentloop.OpReviewMissingSources, now, map[string]any{
				"missing_count": 2,
				"check_ids":     []any{"storage.smart", "boot.bcd"},
			}),
		},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 2, FindingIDs: []string{"evidence.insufficient"},
			ForbiddenClaims:      []string{"hardware is fine", "os is healthy", "powershell"},
			RequiredEvidenceRefs: []string{"checks[1].status"},
			AllowedOperationIDs:  []string{agentloop.OpReviewMissingSources},
		},
	}
}

func corruptWindows(now time.Time) agenteval.Fixture {
	report := baseReport("corrupt-windows", now)
	report.Installations[0].Version = nil
	report.Checks = append(report.Checks, diagnostics.Check{
		ID: "windows.hive-readable", Status: "error", Summary: "SOFTWARE hive unreadable",
	})
	id := report.ReportID
	finding := diagnosis.Finding{
		ID: "windows.corrupt-install", Title: "Windows installation metadata looks corrupt or unreadable",
		Severity: diagnosis.SeverityCritical, Confidence: diagnosis.ConfidenceMedium,
		Rationale: "Offline hive read failed; treat the install as damaged until verified.",
	}
	return agenteval.Fixture{
		ID: "corrupt-windows", Description: "Unreadable offline Windows hive / missing version metadata.",
		Report: report, Session: baseSession(id, now),
		ProviderRounds: []agentloop.ProviderProposal{
			completed(id, now, 1, finding, agentloop.OpIdentifyWindowsInstallation,
				"No repair command is emitted; technician confirmation remains mandatory.",
				"windows_installations[0].root", "checks[1].status"),
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateCompleted, FinalRound: 1, FindingIDs: []string{"windows.corrupt-install"},
			ForbiddenClaims:      []string{"sfc /scannow", "dism /online", "powershell"},
			RequiredEvidenceRefs: []string{"checks[1].status"},
			AllowedOperationIDs:  []string{agentloop.OpIdentifyWindowsInstallation},
		},
	}
}

func intPtr(value int) *int          { return &value }
func uint64Ptr(value uint64) *uint64 { return &value }
