package agenteval_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agenteval"
	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

// TestPolicyRegressionHarness is the dedicated policy regression suite for the
// agent loop. It stays separate from scenario fixtures so regressions in
// allowlists, path binding, and audit behavior are obvious in CI.
func TestPolicyRegressionHarness(t *testing.T) {
	now := time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC)
	fixtures := []agenteval.Fixture{
		policyDuplicateRequest(now),
		policyInventedEvidence(now),
		policyForbiddenRoot(now),
	}
	report := agenteval.RunPolicyRegressionFixtures(context.Background(), fixtures, now)
	if report.Harness != "policy-regression" {
		t.Fatalf("harness = %q", report.Harness)
	}
	if report.Failed != 0 {
		t.Fatalf("policy regression failures: %+v", report.Results)
	}
	out := filepath.Join(t.TempDir(), "policy-regression.json")
	if err := agenteval.WriteReport(out, report); err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}
}

func policyDuplicateRequest(now time.Time) agenteval.Fixture {
	report := basePolicyReport("policy-duplicate", now)
	sess := basePolicySession(report.ReportID, now)
	req := agentloop.EvidenceRequest{
		ID: "req-1", Operation: agentloop.OpInspectNetworkStatus, Arguments: map[string]any{},
		Reason: "Need adapter status.", ExpectedInformation: "Status enum.",
		PrivacyClass: agentloop.PrivacyNetworkStatus, TimeoutSeconds: 10,
	}
	return agenteval.Fixture{
		ID: "policy-duplicate-request", Description: "Repeated evidence fingerprints fail closed with audit.",
		Report: report, Session: sess,
		ProviderRounds: []agentloop.Result{
			{
				State: agentloop.StateNeedsMoreEvidence, EvidenceRequests: []agentloop.EvidenceRequest{req},
				Limitations: []string{"Need one observation."},
			},
			{
				State: agentloop.StateNeedsMoreEvidence,
				EvidenceRequests: []agentloop.EvidenceRequest{{
					ID: "req-2", Operation: agentloop.OpInspectNetworkStatus, Arguments: map[string]any{},
					Reason: "Repeat the same observation.", ExpectedInformation: "Status enum.",
					PrivacyClass: agentloop.PrivacyNetworkStatus, TimeoutSeconds: 10,
				}},
				Limitations: []string{"Duplicate should fail."},
			},
		},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{
			agentloop.CanonicalRequestKey(req): {
				Facts: map[string]any{"status": "connected"}, EvidenceRefs: []string{"hardware.network_adapters[0].status"},
			},
		},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateFailed, FinalRound: 2, FindingIDs: []string{},
			ForbiddenClaims: []string{"powershell"}, RequiredEvidenceRefs: []string{},
			AllowedOperationIDs: []string{agentloop.OpInspectNetworkStatus},
			FailureCode:         "invalid_provider_result",
		},
	}
}

func policyInventedEvidence(now time.Time) agenteval.Fixture {
	report := basePolicyReport("policy-invented", now)
	sess := basePolicySession(report.ReportID, now)
	assessment := diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion, ReportID: report.ReportID, GeneratedAt: now,
		Mode: diagnosis.ModeOnlineAgent,
		Summary: diagnosis.Summary{
			Headline: "Invented path", HighestSeverity: diagnosis.SeverityWarning, FindingCount: 1,
		},
		Findings: []diagnosis.Finding{{
			ID: "invented.path", Title: "Invented evidence path", Severity: diagnosis.SeverityWarning,
			Confidence: diagnosis.ConfidenceLow, Rationale: "This path does not exist.",
			EvidenceRefs: []string{"storage.secret_key"}, SourceRefs: []string{},
		}},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID: "step-review", Title: "Review sources", Operation: agentloop.OpReviewMissingSources,
			Risk: diagnosis.RiskReadOnly, RequiresConfirmation: false, Rationale: "Stay read-only.",
		}},
		Limitations: []string{"Should be rejected by gateway validation."},
		Sources:     []diagnosis.Source{},
	}
	return agenteval.Fixture{
		ID: "policy-invented-evidence", Description: "Completed assessments cannot invent evidence refs.",
		Report: report, Session: sess,
		ProviderRounds: []agentloop.Result{{
			State: agentloop.StateCompleted, Assessment: &assessment,
			EvidenceRequests: []agentloop.EvidenceRequest{},
			Limitations:      []string{"Should be rejected by gateway validation."},
		}},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateFailed, FinalRound: 1, FindingIDs: []string{},
			ForbiddenClaims: []string{"powershell"}, RequiredEvidenceRefs: []string{},
			AllowedOperationIDs: []string{},
			FailureCode:         "invalid_provider_result",
		},
	}
}

func policyForbiddenRoot(now time.Time) agenteval.Fixture {
	report := basePolicyReport("policy-root", now)
	report.Installations = []diagnostics.Installation{{
		Root: "C:\\Windows", SystemHive: "C:\\Windows\\System32\\config\\SYSTEM",
		SoftwareHive: "C:\\Windows\\System32\\config\\SOFTWARE",
	}}
	sess := basePolicySession(report.ReportID, now)
	return agenteval.Fixture{
		ID: "policy-forbidden-root", Description: "UNC and undiscovered roots are rejected.",
		Report: report, Session: sess,
		ProviderRounds: []agentloop.Result{{
			State: agentloop.StateNeedsMoreEvidence,
			EvidenceRequests: []agentloop.EvidenceRequest{{
				ID: "req-root", Operation: agentloop.OpIdentifyWindowsInstallation,
				Arguments:           map[string]any{"root": `\\server\share\Windows`},
				Reason:              "Should reject UNC root.",
				ExpectedInformation: "Installation metadata.",
				PrivacyClass:        agentloop.PrivacyMachineInventory,
				TimeoutSeconds:      15,
			}},
			Limitations: []string{"Path policy."},
		}},
		EvidenceCatalog: map[string]agentloop.EvidencePayload{},
		Expected: agenteval.Expectation{
			FinalState: agentloop.StateFailed, FinalRound: 1, FindingIDs: []string{},
			ForbiddenClaims: []string{"powershell"}, RequiredEvidenceRefs: []string{},
			AllowedOperationIDs: []string{},
			FailureCode:         "invalid_provider_result",
		},
	}
}

func basePolicyReport(id string, now time.Time) diagnostics.Report {
	return diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      "report-" + id,
		CollectedAt:   now,
		Collector:     diagnostics.Collector{Name: "effexorwinpe-collector", Version: "policy"},
		Environment:   diagnostics.Environment{RuntimeOS: "windows", RuntimeArch: "amd64"},
		Hardware: diagnostics.Hardware{
			FirmwareMode: "uefi",
			Memory:       diagnostics.Memory{TotalPhysicalBytes: 8 << 30},
			NetworkAdapters: []diagnostics.NetworkAdapter{{
				Name: "Example NIC", Status: "connected",
			}},
		},
		Storage: diagnostics.Storage{
			Disks:              []diagnostics.Disk{{Number: 0, FriendlyName: "Example", SizeBytes: 100, HealthStatus: "Healthy"}},
			DriveHealth:        []diagnostics.DriveHealth{},
			Partitions:         []diagnostics.Partition{},
			BitLockerVolumes:   []diagnostics.BitLockerVolume{},
			BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusOK},
		},
		Boot:          diagnostics.Boot{FirmwareMode: "uefi", BCDStores: []diagnostics.BCDStore{}},
		Installations: []diagnostics.Installation{},
		Checks:        []diagnostics.Check{{ID: "collector.runtime", Status: "ok", Summary: "ok"}},
		Privacy:       diagnostics.Privacy{ExcludedByDefault: []string{"hostname"}},
	}
}

func basePolicySession(reportID string, now time.Time) session.Session {
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

func TestPolicyRegressionNamesAreStable(t *testing.T) {
	if !strings.Contains("policy-regression", "policy") {
		t.Fatal("harness name drift")
	}
}
