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
	if err := ValidateEvidenceRequest(request, report); err != nil {
		t.Fatalf("ValidateEvidenceRequest() error = %v", err)
	}
	request.Arguments = map[string]any{"device_id": `\\.\PhysicalDrive0`}
	if err := ValidateEvidenceRequest(request, report); err == nil {
		t.Fatal("expected forbidden device namespace rejection")
	}
	request.Operation = "run_powershell"
	request.Arguments = map[string]any{}
	if err := ValidateEvidenceRequest(request, report); err == nil {
		t.Fatal("expected unknown operation rejection")
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

	provider := &scriptedProvider{rounds: []Result{
		{
			State: StateNeedsMoreEvidence,
			EvidenceRequests: []EvidenceRequest{{
				ID:                  "req-1",
				Operation:           OpInspectStorageHealth,
				Arguments:           map[string]any{},
				Reason:              "Need drive health details.",
				ExpectedInformation: "Health status and counters.",
				PrivacyClass:        PrivacyStorageHealth,
				TimeoutSeconds:      30,
			}},
			Limitations: []string{"Waiting for storage evidence."},
		},
		{
			State: StateNeedsMoreEvidence,
			EvidenceRequests: []EvidenceRequest{{
				ID:                  "req-2",
				Operation:           OpInspectStorageHealth,
				Arguments:           map[string]any{},
				Reason:              "Repeat the same ask.",
				ExpectedInformation: "Health status and counters.",
				PrivacyClass:        PrivacyStorageHealth,
				TimeoutSeconds:      30,
			}},
			Limitations: []string{"Should be rejected as duplicate."},
		},
	}}
	collector := mapCollector{
		OpInspectStorageHealth: EvidencePayload{
			Facts:        map[string]any{"health_status": "Warning"},
			EvidenceRefs: []string{"storage.disks[0].health_status"},
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
	if result.Failure == nil || result.Failure.Code != "invalid_provider_result" {
		t.Fatalf("failure = %#v", result.Failure)
	}
	failedEvents := 0
	sawDuplicate := false
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
		t.Fatalf("loop_failed count = %d, want 1 in %#v", failedEvents, result.AuditTimeline)
	}
}

func TestLoopCompletesAfterEvidence(t *testing.T) {
	now := time.Unix(1_700_000_100, 0).UTC()
	report := sampleReport("report-loop-2")
	sess := sampleSession(t, report.ReportID, now)
	assessment := sampleAssessment(report.ReportID, now)

	provider := &scriptedProvider{rounds: []Result{
		{
			State: StateNeedsMoreEvidence,
			EvidenceRequests: []EvidenceRequest{{
				ID:                  "req-net",
				Operation:           OpInspectNetworkStatus,
				Arguments:           map[string]any{},
				Reason:              "Confirm link and DHCP state.",
				ExpectedInformation: "Adapter status codes.",
				PrivacyClass:        PrivacyNetworkStatus,
				TimeoutSeconds:      20,
			}},
			Limitations: []string{"Network evidence incomplete."},
		},
		{
			State:       StateCompleted,
			Assessment:  &assessment,
			Limitations: []string{"Completed from local evidence only."},
		},
	}}
	collector := mapCollector{
		OpInspectNetworkStatus: EvidencePayload{
			Facts:        map[string]any{"dhcp": "none"},
			EvidenceRefs: []string{"hardware.network_adapters[0].status"},
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
	if result.State != StateCompleted {
		t.Fatalf("state = %q", result.State)
	}
	if result.Round != 2 {
		t.Fatalf("round = %d", result.Round)
	}
	if len(result.AuditTimeline) < 4 {
		t.Fatalf("audit timeline too short: %#v", result.AuditTimeline)
	}
}

func TestMaxRoundsBlocksUsesConfiguredLimit(t *testing.T) {
	now := time.Unix(1_700_000_200, 0).UTC()
	report := sampleReport("report-loop-3")
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []Result{
		needsEvidence(OpInspectBootFirmware, PrivacyBootConfig, "req-a"),
		needsEvidence(OpInspectPartitionLayout, PrivacyMachineInventory, "req-b"),
		needsEvidence(OpReviewMissingSources, PrivacyMachineInventory, "req-c"),
	}}
	collector := mapCollector{
		OpInspectBootFirmware:    EvidencePayload{Facts: map[string]any{"mode": "uefi"}, EvidenceRefs: []string{}},
		OpInspectPartitionLayout: EvidencePayload{Facts: map[string]any{"partitions": 4}, EvidenceRefs: []string{}},
		OpReviewMissingSources:   EvidencePayload{Facts: map[string]any{"missing": 1}, EvidenceRefs: []string{}},
	}
	loop := Loop{
		Provider:  provider,
		Collector: collector,
		Options:   Options{Now: func() time.Time { return now }, Timeout: time.Minute, MaxRounds: 3},
	}
	result, err := loop.Run(context.Background(), report, sess)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.State != StateBlocked {
		t.Fatalf("state = %q, want blocked", result.State)
	}
	if result.Block == nil || result.Block.Code != "max_rounds_exceeded" {
		t.Fatalf("block = %#v", result.Block)
	}
	if !strings.Contains(result.Block.Message, "3 rounds") {
		t.Fatalf("block message = %q", result.Block.Message)
	}
}

func TestEvidenceRequestTimeoutPropagates(t *testing.T) {
	now := time.Unix(1_700_000_300, 0).UTC()
	report := sampleReport("report-loop-4")
	sess := sampleSession(t, report.ReportID, now)
	provider := &scriptedProvider{rounds: []Result{
		needsEvidence(OpInspectNetworkStatus, PrivacyNetworkStatus, "req-timeout"),
	}}
	collector := timeoutCollector{}
	loop := Loop{
		Provider:  provider,
		Collector: collector,
		Options:   Options{Now: func() time.Time { return now }, Timeout: time.Minute},
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
	rounds []Result
	index  int
}

func (provider *scriptedProvider) Propose(context.Context, RoundInput) (Result, error) {
	if provider.index >= len(provider.rounds) {
		return Result{}, context.Canceled
	}
	result := provider.rounds[provider.index]
	provider.index++
	if result.EvidenceRequests == nil {
		result.EvidenceRequests = []EvidenceRequest{}
	}
	return result, nil
}

type mapCollector map[string]EvidencePayload

func (collector mapCollector) Collect(_ context.Context, request EvidenceRequest) (EvidencePayload, error) {
	payload, ok := collector[request.Operation]
	if !ok {
		return EvidencePayload{}, context.Canceled
	}
	payload.RequestID = request.ID
	payload.Operation = request.Operation
	if payload.Facts == nil {
		payload.Facts = map[string]any{}
	}
	if payload.EvidenceRefs == nil {
		payload.EvidenceRefs = []string{}
	}
	return payload, nil
}

type timeoutCollector struct{}

func (timeoutCollector) Collect(ctx context.Context, _ EvidenceRequest) (EvidencePayload, error) {
	if _, ok := ctx.Deadline(); !ok {
		return EvidencePayload{}, context.Canceled
	}
	return EvidencePayload{}, context.DeadlineExceeded
}

func needsEvidence(operation, privacy, id string) Result {
	return Result{
		State: StateNeedsMoreEvidence,
		EvidenceRequests: []EvidenceRequest{{
			ID:                  id,
			Operation:           operation,
			Arguments:           map[string]any{},
			Reason:              "Need additional local evidence.",
			ExpectedInformation: "Structured read-only facts.",
			PrivacyClass:        privacy,
			TimeoutSeconds:      1,
		}},
		Limitations: []string{"Evidence incomplete."},
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
	if !strings.Contains(left, OpInspectStorageHealth) {
		t.Fatalf("key = %q", left)
	}
}
