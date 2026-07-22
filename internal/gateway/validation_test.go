package gateway

import (
	"strings"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

func testDiagnosisRequest(t *testing.T) DiagnosisRequest {
	t.Helper()
	now := time.Unix(100, 0).UTC()
	value, err := session.New("report-1", now)
	if err != nil {
		t.Fatalf("session.New() error = %v", err)
	}
	return DiagnosisRequest{
		DiagnosticReport: diagnostics.Report{
			SchemaVersion: diagnostics.SchemaVersion,
			ReportID:      "report-1",
			CollectedAt:   now,
			Environment: diagnostics.Environment{
				RuntimeOS: "windows",
				Hostname:  "client-laptop",
			},
			Storage: diagnostics.Storage{Disks: []diagnostics.Disk{{Number: 0, HealthStatus: "Healthy"}}},
			Checks:  []diagnostics.Check{{ID: "collector.runtime", Status: "ok", Summary: "running"}},
			Privacy: diagnostics.Privacy{ContainsPersonalData: true, ExcludedByDefault: []string{"hostname"}},
		},
		Session:            value,
		TechnicianApproved: true,
	}
}

func testOnlineAssessment(request DiagnosisRequest) diagnosis.Assessment {
	const sourceURL = "https://learn.microsoft.com/windows-hardware/drivers/"
	return diagnosis.Assessment{
		SchemaVersion: diagnosis.SchemaVersion,
		ReportID:      request.DiagnosticReport.ReportID,
		GeneratedAt:   time.Unix(200, 0).UTC(),
		Mode:          diagnosis.ModeOnlineAgent,
		Summary:       diagnosis.Summary{Headline: "Накопитель виден", HighestSeverity: diagnosis.SeverityInfo, FindingCount: 1},
		Findings: []diagnosis.Finding{{
			ID:           "storage.visible",
			Title:        "Накопитель обнаружен",
			Severity:     diagnosis.SeverityInfo,
			Confidence:   diagnosis.ConfidenceHigh,
			Rationale:    "Отчёт содержит первый диск.",
			EvidenceRefs: []string{"storage.disks[0].health_status"},
			SourceRefs:   []string{sourceURL},
		}},
		Questions: []diagnosis.Question{},
		NextSteps: []diagnosis.NextStep{{
			ID:                   "inspect-storage",
			Title:                "Проверить показатели накопителя",
			Operation:            "inspect_storage_health",
			Risk:                 diagnosis.RiskReadOnly,
			RequiresConfirmation: false,
			Rationale:            "Сверить базовые данные с диагностикой производителя.",
		}},
		Limitations: []string{"Это диагностическая гипотеза, а не подтверждение исправности."},
		Sources: []diagnosis.Source{{
			Title:  "Microsoft driver documentation",
			URL:    sourceURL,
			Domain: "learn.microsoft.com",
		}},
	}
}

func TestEvidenceReferencesExcludeHostnameAndIncludeSessionNamespace(t *testing.T) {
	request := testDiagnosisRequest(t)
	paths, err := EvidenceReferences(request)
	if err != nil {
		t.Fatalf("EvidenceReferences() error = %v", err)
	}
	foundStorage := false
	foundSession := false
	for _, path := range paths {
		if path == "environment.hostname" {
			t.Fatal("hostname remained in the model evidence catalog")
		}
		if path == "storage.disks[0].health_status" {
			foundStorage = true
		}
		if path == "session.symptoms" {
			foundSession = true
		}
		if path == "session.session_id" || strings.HasPrefix(path, "session.latest_assessment") {
			t.Fatalf("derived session metadata entered evidence catalog: %s", path)
		}
	}
	if !foundStorage || !foundSession {
		t.Fatalf("missing expected evidence paths: storage=%v session=%v", foundStorage, foundSession)
	}
}

func TestValidateOnlineAssessmentAcceptsGroundedReadOnlyResult(t *testing.T) {
	request := testDiagnosisRequest(t)
	if err := ValidateDiagnosisRequest(request); err != nil {
		t.Fatalf("ValidateDiagnosisRequest() error = %v", err)
	}
	if err := ValidateOnlineAssessment(testOnlineAssessment(request), request); err != nil {
		t.Fatalf("ValidateOnlineAssessment() error = %v", err)
	}
}

func TestValidateOnlineAssessmentRejectsInventedEvidenceSourceAndOperation(t *testing.T) {
	request := testDiagnosisRequest(t)
	tests := []struct {
		name   string
		mutate func(*diagnosis.Assessment)
	}{
		{name: "evidence", mutate: func(value *diagnosis.Assessment) {
			value.Findings[0].EvidenceRefs = []string{"storage.disks[9].secret"}
		}},
		{name: "source", mutate: func(value *diagnosis.Assessment) {
			value.Findings[0].SourceRefs = []string{"https://example.invalid/invented"}
		}},
		{name: "operation", mutate: func(value *diagnosis.Assessment) { value.NextSteps[0].Operation = "run_powershell" }},
		{name: "mutation", mutate: func(value *diagnosis.Assessment) { value.NextSteps[0].Risk = diagnosis.RiskChangesSystem }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := testOnlineAssessment(request)
			test.mutate(&value)
			if err := ValidateOnlineAssessment(value, request); err == nil {
				t.Fatal("ValidateOnlineAssessment() error = nil")
			}
		})
	}
}
