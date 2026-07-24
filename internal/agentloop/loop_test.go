package agentloop

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

func TestValidateEvidenceRequestAllowlistAndContext(t *testing.T) {
	report := sampleReport("report-ops-1")
	request := EvidenceRequest{
		ID:                  "req-storage",
		Operation:           OpInspectStorageHealth,
		Arguments:           map[string]any{"device_id": "0"},
		Reason:              "Need SMART counters.",
		ExpectedInformation: "Temperature and error counters.",
		PrivacyClass:        PrivacyStorageHealth,
		TimeoutSeconds:      30,
	}
	if err := ValidateEvidenceRequest(&request, report); err != nil {
		t.Fatalf("ValidateEvidenceRequest() error = %v", err)
	}
	request.Arguments = map[string]any{"device_id": `\\.\PhysicalDrive0`}
	if err := ValidateEvidenceRequest(&request, report); err == nil {
		t.Fatal("expected forbidden device namespace rejection")
	}
	request.Operation = "run_powershell"
	request.Arguments = map[string]any{}
	if err := ValidateEvidenceRequest(&request, report); err == nil {
		t.Fatal("expected unknown operation rejection")
	}
}

func TestWindowsPathNormalization(t *testing.T) {
	report := sampleReport("report-paths")
	report.Installations = []diagnostics.Installation{{Root: `C:\Windows`}}
	request := EvidenceRequest{
		ID: "req-root", Operation: OpIdentifyWindowsInstallation,
		Arguments:           map[string]any{"root": `c:/windows/`},
		Reason:              "Case and slash variants must match.",
		ExpectedInformation: "Installation metadata.",
		PrivacyClass:        PrivacyMachineInventory,
		TimeoutSeconds:      15,
	}
	if err := ValidateEvidenceRequest(&request, report); err != nil {
		t.Fatalf("ValidateEvidenceRequest() error = %v", err)
	}
	if got, _ := request.Arguments["root"].(string); got != `C:\Windows` {
		t.Fatalf("normalized root = %q", got)
	}
}

func TestSanitizeAgentContextStripsHostnameAndSessionSecrets(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	report := sampleReport("report-sanitize")
	report.Environment.Hostname = "client-laptop"
	report.Privacy.ContainsPersonalData = true
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)
	sess.LatestAssessment = &assessment
	sanitized := NewSanitizedAgentContext(report, sess)
	if sanitized.Report.Environment.Hostname != "" {
		t.Fatalf("hostname leaked: %q", sanitized.Report.Environment.Hostname)
	}
	if sanitized.Report.Privacy.ContainsPersonalData {
		t.Fatal("contains_personal_data leaked")
	}
	if sanitized.Session.LatestAssessment != nil {
		t.Fatal("latest assessment leaked")
	}
	if sanitized.Session.Events != nil {
		t.Fatal("session events leaked")
	}
}

func TestProviderDoesNotReceiveRawHostname(t *testing.T) {
	now := time.Unix(1_700_000_400, 0).UTC()
	report := sampleReport("report-sanitize-loop")
	report.Environment.Hostname = "secret-host"
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []ProviderProposal{
		completeProposal(report.ReportID, now, 1, sampleAssessment(report.ReportID, now)),
	}}
	loop := Loop{Provider: provider, Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute}}
	if _, err := loop.Run(context.Background(), report, sess); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if provider.lastInput.Context.Report.Environment.Hostname != "" {
		t.Fatalf("provider saw hostname %q", provider.lastInput.Context.Report.Environment.Hostname)
	}
}

func TestRejectCommandText(t *testing.T) {
	if err := RejectCommandText("rationale", "Review vendor SMART thresholds."); err != nil {
		t.Fatalf("unexpected rejection: %v", err)
	}
	if err := RejectCommandText("rationale", "Run powershell Get-Disk"); err == nil {
		t.Fatal("expected powershell rejection")
	}
}

func TestLoopCompletesAndBlocksDuplicateEvidence(t *testing.T) {
	now := time.Unix(1_700_000_000, 0).UTC()
	report := sampleReport("report-loop-1")
	sess := sampleSession(t, report.ReportID, now)

	provider := &scriptedProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-1", Operation: OpInspectNetworkStatus, Arguments: map[string]any{},
			Reason: "Need drive health details.", ExpectedInformation: "Health status and counters.",
			PrivacyClass: PrivacyNetworkStatus, TimeoutSeconds: 30,
		}),
		needsProposal(report.ReportID, now, 2, EvidenceRequest{
			ID: "req-2", Operation: OpInspectNetworkStatus, Arguments: map[string]any{},
			Reason: "Repeat the same ask.", ExpectedInformation: "Health status and counters.",
			PrivacyClass: PrivacyNetworkStatus, TimeoutSeconds: 30,
		}),
	}}
	collector := mapCollector{
		OpInspectNetworkStatus: EvidencePayload{
			RequestID: "req-1", Operation: OpInspectNetworkStatus, CollectedAt: now,
			Facts: map[string]any{
				"adapters": []any{map[string]any{"name": "Example NIC", "status": "connected"}},
			},
			EvidenceRefs: []string{"invented.ref"},
		},
	}
	loop := Loop{
		Provider:  provider,
		Collector: collector,
		Options:   Options{Now: func() time.Time { return now }, Timeout: time.Minute},
	}
	result, err := loop.Run(context.Background(), report, sess)
	if err == nil {
		t.Fatal("expected duplicate evidence rejection")
	}
	if result.State != StateFailed {
		t.Fatalf("state = %q, want failed", result.State)
	}
	sawDuplicate := false
	failedEvents := 0
	for _, event := range result.AuditTimeline {
		if event.Kind == AuditDuplicateRequestRejected {
			sawDuplicate = true
		}
		if event.Kind == AuditLoopFailed {
			failedEvents++
		}
	}
	if !sawDuplicate {
		t.Fatalf("missing duplicate_request_rejected in %#v", result.AuditTimeline)
	}
	if failedEvents != 1 {
		t.Fatalf("loop_failed count = %d", failedEvents)
	}
}

func TestInventedCollectorEvidenceRefsAreReplaced(t *testing.T) {
	now := time.Unix(1_700_000_500, 0).UTC()
	report := sampleReport("report-refs")
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)
	provider := &capturingProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-net", Operation: OpInspectNetworkStatus, Arguments: map[string]any{},
			Reason: "Need adapters.", ExpectedInformation: "Adapter status.",
			PrivacyClass: PrivacyNetworkStatus, TimeoutSeconds: 20,
		}),
		completeProposal(report.ReportID, now, 2, assessment),
	}}
	collector := mapCollector{
		OpInspectNetworkStatus: EvidencePayload{
			RequestID: "req-net", Operation: OpInspectNetworkStatus, CollectedAt: now,
			Facts: map[string]any{
				"adapters": []any{map[string]any{"name": "Example NIC", "status": "connected"}},
			},
			EvidenceRefs: []string{"storage.secret_key", "invented.path"},
		},
	}
	loop := Loop{Provider: provider, Collector: collector, Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute}}
	if _, err := loop.Run(context.Background(), report, sess); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(provider.secondInput.PriorEvidence) != 1 {
		t.Fatalf("prior evidence = %#v", provider.secondInput.PriorEvidence)
	}
	refs := provider.secondInput.PriorEvidence[0].EvidenceRefs
	for _, ref := range refs {
		if strings.Contains(ref, "invented") || ref == "storage.secret_key" {
			t.Fatalf("invented collector ref survived: %v", refs)
		}
		if !strings.HasPrefix(ref, "evidence."+OpInspectNetworkStatus) {
			t.Fatalf("ref %q escaped operation namespace", ref)
		}
	}
}

func TestEncryptionEvidenceIsNotUploadedToProvider(t *testing.T) {
	now := time.Unix(1_700_000_600, 0).UTC()
	report := sampleReport("report-bitlocker")
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)
	provider := &capturingProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-bl", Operation: OpReviewBitLockerAccess, Arguments: map[string]any{},
			Reason: "Need inventory status.", ExpectedInformation: "BitLocker availability.",
			PrivacyClass: PrivacyEncryptionStatus, TimeoutSeconds: 20,
		}),
		completeProposal(report.ReportID, now, 2, assessment),
	}}
	collector := mapCollector{
		OpReviewBitLockerAccess: EvidencePayload{
			RequestID: "req-bl", Operation: OpReviewBitLockerAccess, CollectedAt: now,
			Facts:        map[string]any{"inventory_status": "unavailable"},
			EvidenceRefs: []string{},
		},
	}
	loop := Loop{Provider: provider, Collector: collector, Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute}}
	if _, err := loop.Run(context.Background(), report, sess); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(provider.secondInput.PriorEvidence) != 0 {
		t.Fatalf("encryption evidence leaked to provider: %#v", provider.secondInput.PriorEvidence)
	}
}

func TestArbitrarySourceURLRejected(t *testing.T) {
	now := time.Unix(1_700_000_700, 0).UTC()
	report := sampleReport("report-source")
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)
	assessment.Sources = []diagnosis.Source{{
		Title: "Invented", URL: "https://evil.example/advice", Domain: "evil.example",
	}}
	assessment.Findings[0].SourceRefs = []string{"https://evil.example/advice"}
	provider := &scriptedProvider{rounds: []ProviderProposal{
		completeProposal(report.ReportID, now, 1, assessment),
	}}
	loop := Loop{Provider: provider, Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute}}
	result, err := loop.Run(context.Background(), report, sess)
	if err == nil {
		t.Fatal("expected invented source rejection")
	}
	if result.Failure == nil || result.Failure.Code != "invalid_provider_result" {
		t.Fatalf("failure = %#v", result.Failure)
	}
}

func TestProviderMissingRequiredFieldsIsNotAutofilled(t *testing.T) {
	now := time.Unix(1_700_000_800, 0).UTC()
	report := sampleReport("report-malformed")
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []ProviderProposal{{
		State:            StateCompleted,
		EvidenceRequests: []EvidenceRequest{},
		RetrievedSources: []diagnosis.Source{},
		Limitations:      []string{"missing ids"},
		Assessment:       &diagnosis.Assessment{},
	}}}
	loop := Loop{Provider: provider, Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute}}
	result, err := loop.Run(context.Background(), report, sess)
	if err == nil {
		t.Fatal("expected malformed proposal rejection")
	}
	if result.Failure == nil || result.Failure.Code != "invalid_provider_result" {
		t.Fatalf("failure = %#v", result.Failure)
	}
}

func TestLoopCompletesAfterEvidence(t *testing.T) {
	now := time.Unix(1_700_000_100, 0).UTC()
	report := sampleReport("report-loop-2")
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)

	provider := &scriptedProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-net", Operation: OpInspectNetworkStatus, Arguments: map[string]any{},
			Reason: "Confirm link and DHCP state.", ExpectedInformation: "Adapter status codes.",
			PrivacyClass: PrivacyNetworkStatus, TimeoutSeconds: 20,
		}),
		completeProposal(report.ReportID, now, 2, assessment),
	}}
	collector := mapCollector{
		OpInspectNetworkStatus: EvidencePayload{
			RequestID: "req-net", Operation: OpInspectNetworkStatus, CollectedAt: now,
			Facts: map[string]any{
				"adapters":   []any{map[string]any{"name": "Example NIC", "status": "connected"}},
				"dhcp_state": "none",
			},
		},
	}
	loop := Loop{
		Provider:  provider,
		Collector: collector,
		Options:   Options{Now: func() time.Time { return now }, Timeout: time.Minute},
	}
	result, err := loop.Run(context.Background(), report, sess)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != StateCompleted || result.Round != 2 {
		t.Fatalf("state=%q round=%d", result.State, result.Round)
	}
}

func TestMaxRoundsBlocksUsesConfiguredLimit(t *testing.T) {
	now := time.Unix(1_700_000_200, 0).UTC()
	report := sampleReport("report-loop-3")
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-a", Operation: OpInspectBootFirmware, Arguments: map[string]any{},
			Reason: "Need firmware mode.", ExpectedInformation: "Firmware mode.",
			PrivacyClass: PrivacyBootConfig, TimeoutSeconds: 15,
		}),
		needsProposal(report.ReportID, now, 2, EvidenceRequest{
			ID: "req-b", Operation: OpInspectPartitionLayout, Arguments: map[string]any{},
			Reason: "Need partitions.", ExpectedInformation: "Partition table.",
			PrivacyClass: PrivacyMachineInventory, TimeoutSeconds: 15,
		}),
		needsProposal(report.ReportID, now, 3, EvidenceRequest{
			ID: "req-c", Operation: OpReviewMissingSources, Arguments: map[string]any{},
			Reason: "Need missing checks.", ExpectedInformation: "Missing count.",
			PrivacyClass: PrivacyMachineInventory, TimeoutSeconds: 15,
		}),
	}}
	collector := mapCollector{
		OpInspectBootFirmware: EvidencePayload{
			RequestID: "req-a", Operation: OpInspectBootFirmware, CollectedAt: now,
			Facts: map[string]any{"firmware_mode": "uefi"},
		},
		OpInspectPartitionLayout: EvidencePayload{
			RequestID: "req-b", Operation: OpInspectPartitionLayout, CollectedAt: now,
			Facts: map[string]any{
				"partitions": []any{map[string]any{"disk_number": 0, "partition_number": 1, "size_bytes": 100}},
			},
		},
		OpReviewMissingSources: EvidencePayload{
			RequestID: "req-c", Operation: OpReviewMissingSources, CollectedAt: now,
			Facts: map[string]any{"missing_count": 1},
		},
	}
	loop := Loop{
		Provider: provider, Collector: collector,
		Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute, MaxRounds: 3},
	}
	result, err := loop.Run(context.Background(), report, sess)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != StateBlocked || result.Block == nil || result.Block.Code != "max_rounds_exceeded" {
		t.Fatalf("result = %#v", result)
	}
	if !strings.Contains(result.Block.Message, "3 rounds") {
		t.Fatalf("block message = %q", result.Block.Message)
	}
}

func TestEvidenceRequestTimeoutPropagates(t *testing.T) {
	now := time.Unix(1_700_000_300, 0).UTC()
	report := sampleReport("report-loop-4")
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []ProviderProposal{
		needsProposal(report.ReportID, now, 1, EvidenceRequest{
			ID: "req-timeout", Operation: OpInspectNetworkStatus, Arguments: map[string]any{},
			Reason: "Need adapters.", ExpectedInformation: "Adapter status.",
			PrivacyClass: PrivacyNetworkStatus, TimeoutSeconds: 1,
		}),
	}}
	loop := Loop{
		Provider: provider, Collector: timeoutCollector{},
		Options: Options{Now: func() time.Time { return now }, Timeout: time.Minute},
	}
	result, err := loop.Run(context.Background(), report, sess)
	if err == nil {
		t.Fatal("expected timeout failure")
	}
	if result.Failure == nil || result.Failure.Code != "evidence_collection_failed" {
		t.Fatalf("failure = %#v", result.Failure)
	}
}

type scriptedProvider struct {
	rounds    []ProviderProposal
	index     int
	lastInput RoundInput
}

func (provider *scriptedProvider) Propose(_ context.Context, input RoundInput) (ProviderProposal, error) {
	provider.lastInput = input
	if provider.index >= len(provider.rounds) {
		return ProviderProposal{}, context.Canceled
	}
	proposal := provider.rounds[provider.index]
	provider.index++
	return proposal, nil
}

type capturingProvider struct {
	rounds      []ProviderProposal
	index       int
	secondInput RoundInput
}

func (provider *capturingProvider) Propose(_ context.Context, input RoundInput) (ProviderProposal, error) {
	if provider.index == 1 {
		provider.secondInput = input
	}
	if provider.index >= len(provider.rounds) {
		return ProviderProposal{}, context.Canceled
	}
	proposal := provider.rounds[provider.index]
	provider.index++
	return proposal, nil
}

type mapCollector map[string]EvidencePayload

func (collector mapCollector) Collect(_ context.Context, request EvidenceRequest) (EvidencePayload, error) {
	payload, ok := collector[request.Operation]
	if !ok {
		return EvidencePayload{}, context.Canceled
	}
	payload.RequestID = request.ID
	payload.Operation = request.Operation
	return payload, nil
}

type timeoutCollector struct{}

func (timeoutCollector) Collect(ctx context.Context, _ EvidenceRequest) (EvidencePayload, error) {
	if _, ok := ctx.Deadline(); !ok {
		return EvidencePayload{}, context.Canceled
	}
	return EvidencePayload{}, context.DeadlineExceeded
}

func needsProposal(reportID string, now time.Time, round int, request EvidenceRequest) ProviderProposal {
	return ProviderProposal{
		SchemaVersion:    SchemaVersion,
		ReportID:         reportID,
		GeneratedAt:      now,
		State:            StateNeedsMoreEvidence,
		Round:            round,
		EvidenceRequests: []EvidenceRequest{request},
		Limitations:      []string{"Evidence incomplete."},
		RetrievedSources: []diagnosis.Source{},
	}
}

func completeProposal(reportID string, now time.Time, round int, assessment diagnosis.Assessment) ProviderProposal {
	assessment.ReportID = reportID
	assessment.GeneratedAt = now
	return ProviderProposal{
		SchemaVersion:    SchemaVersion,
		ReportID:         reportID,
		GeneratedAt:      now,
		State:            StateCompleted,
		Round:            round,
		Assessment:       &assessment,
		EvidenceRequests: []EvidenceRequest{},
		Limitations:      []string{"Completed from local evidence only."},
		RetrievedSources: []diagnosis.Source{},
	}
}

func sampleReport(reportID string) diagnostics.Report {
	return diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      reportID,
		CollectedAt:   time.Unix(1_700_000_000, 0).UTC(),
		Collector:     diagnostics.Collector{Name: "effexorwinpe-collector", Version: "test"},
		Environment:   diagnostics.Environment{RuntimeOS: "windows", RuntimeArch: "amd64"},
		Hardware: diagnostics.Hardware{
			FirmwareMode: "uefi",
			Memory:       diagnostics.Memory{TotalPhysicalBytes: 8 << 30},
			NetworkAdapters: []diagnostics.NetworkAdapter{{
				Name:   "Example NIC",
				Status: "connected",
			}},
		},
		Storage: diagnostics.Storage{
			Disks: []diagnostics.Disk{{
				Number:       0,
				FriendlyName: "Example Disk",
				SizeBytes:    512 << 30,
				HealthStatus: "Healthy",
			}},
			DriveHealth:      []diagnostics.DriveHealth{},
			Partitions:       []diagnostics.Partition{},
			BitLockerVolumes: []diagnostics.BitLockerVolume{},
			BitLockerInventory: diagnostics.BitLockerInventory{
				Status: diagnostics.BitLockerStatusOK,
			},
		},
		Boot:          diagnostics.Boot{FirmwareMode: "uefi", BCDStores: []diagnostics.BCDStore{}},
		Installations: []diagnostics.Installation{},
		Checks:        []diagnostics.Check{{ID: "collector.runtime", Status: "ok", Summary: "ok"}},
		Privacy:       diagnostics.Privacy{ExcludedByDefault: []string{"hostname"}},
	}
}

func sampleSession(t *testing.T, reportID string, now time.Time) session.Session {
	t.Helper()
	value, err := session.New(reportID, now)
	if err != nil {
		t.Fatalf("session.New() error = %v", err)
	}
	return value
}

func sampleAssessment(reportID string, now time.Time) diagnosis.Assessment {
	return diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      reportID,
		GeneratedAt:   now,
		Mode:          diagnosis.ModeOnlineAgent,
		Summary: diagnosis.Summary{
			Headline:        "Network evidence reviewed",
			HighestSeverity: diagnosis.SeverityWarning,
			FindingCount:    1,
		},
		Findings: []diagnosis.Finding{{
			ID:           "network.no-dhcp",
			Title:        "DHCP lease was not observed",
			Severity:     diagnosis.SeverityWarning,
			Confidence:   diagnosis.ConfidenceMedium,
			Rationale:    "Adapter status does not show a lease.",
			EvidenceRefs: []string{"hardware.network_adapters[0].status"},
			SourceRefs:   []string{},
		}},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID:                   "review-sources",
			Title:                "Review missing diagnostic sources",
			Operation:            OpReviewMissingSources,
			Risk:                 diagnosis.RiskReadOnly,
			RequiresConfirmation: false,
			Rationale:            "Confirm collector coverage before repair planning.",
		}},
		Limitations: []string{"This is not a clean bill of health."},
		Sources:     []diagnosis.Source{},
	}
}

func TestCanonicalRequestKeyStable(t *testing.T) {
	left := CanonicalRequestKey(EvidenceRequest{
		Operation: OpInspectStorageHealth,
		Arguments: map[string]any{"device_id": "0", "unused": nil},
	})
	right := CanonicalRequestKey(EvidenceRequest{
		Operation: OpInspectStorageHealth,
		Arguments: map[string]any{"unused": nil, "device_id": "0"},
	})
	if left != right {
		t.Fatalf("keys differ: %q vs %q", left, right)
	}
}
