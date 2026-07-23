package agenteval_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agenteval"
	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
)

func TestEvalFixtures(t *testing.T) {
	dir := filepath.Join("testdata", "fixtures")
	fixtures, err := agenteval.LoadFixtures(dir)
	if err != nil {
		t.Fatalf("LoadFixtures() error = %v", err)
	}
	if len(fixtures) < 10 {
		t.Fatalf("fixture count = %d, want at least 10", len(fixtures))
	}

	now := time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC)
	report := agenteval.RunFixtures(context.Background(), fixtures, now)
	outPath := filepath.Join(t.TempDir(), "eval-report.json")
	if envPath := os.Getenv("EFFEXORWINPE_EVAL_OUT"); envPath != "" {
		outPath = envPath
	}
	if err := agenteval.WriteReport(outPath, report); err != nil {
		t.Fatalf("WriteReport() error = %v", err)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read eval report: %v", err)
	}
	var decoded agenteval.Report
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("eval report is not machine-readable JSON: %v", err)
	}
	if decoded.SchemaVersion != agenteval.ReportSchemaVersion {
		t.Fatalf("report schema = %q", decoded.SchemaVersion)
	}
	if report.Failed != 0 {
		t.Fatalf("eval failures: %+v", report.Results)
	}
	if report.Passed != len(fixtures) {
		t.Fatalf("passed = %d, fixtures = %d", report.Passed, len(fixtures))
	}

	seen := map[string]struct{}{}
	required := []string{
		"healthy", "failing-hdd", "missing-smart", "bitlocker-unavailable",
		"multiple-windows", "bcd-mismatch", "no-dhcp", "dual-boot",
		"insufficient-evidence", "corrupt-windows",
	}
	for _, fixture := range fixtures {
		seen[fixture.ID] = struct{}{}
		for _, round := range fixture.ProviderRounds {
			for _, request := range round.EvidenceRequests {
				if !agentloop.IsAllowedEvidenceOperation(request.Operation) {
					t.Fatalf("fixture %s uses non-allowlisted operation %q", fixture.ID, request.Operation)
				}
			}
		}
	}
	for _, id := range required {
		if _, ok := seen[id]; !ok {
			t.Fatalf("missing required scenario %q", id)
		}
	}
}

func TestMockProviderIsDeterministic(t *testing.T) {
	provider := agenteval.NewMockProvider([]agentloop.Result{{
		State:       agentloop.StateFailed,
		Failure:     &agentloop.StatusDetail{Code: "scripted", Message: "deterministic failure"},
		Limitations: []string{"fixture"},
	}})
	first, err := provider.Propose(context.Background(), agentloop.RoundInput{Round: 1})
	if err != nil {
		t.Fatalf("Propose() error = %v", err)
	}
	if first.State != agentloop.StateFailed {
		t.Fatalf("state = %q", first.State)
	}
	if _, err := provider.Propose(context.Background(), agentloop.RoundInput{Round: 2}); err == nil {
		t.Fatal("expected mock provider exhaustion error")
	}
}
