package agenteval

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

const ReportSchemaVersion = "0.1.0"

// Fixture is one anonymized, reproducible agent-loop scenario.
type Fixture struct {
	ID              string                               `json:"id"`
	Description     string                               `json:"description"`
	Report          diagnostics.Report                   `json:"report"`
	Session         session.Session                      `json:"session"`
	ProviderRounds  []agentloop.Result                   `json:"provider_rounds"`
	EvidenceCatalog map[string]agentloop.EvidencePayload `json:"evidence_catalog"`
	Expected        Expectation                          `json:"expected"`
}

type Expectation struct {
	FinalState           string   `json:"final_state"`
	FindingIDs           []string `json:"finding_ids"`
	ForbiddenClaims      []string `json:"forbidden_claims"`
	RequiredEvidenceRefs []string `json:"required_evidence_refs"`
	AllowedOperationIDs  []string `json:"allowed_operation_ids"`
}

// CaseResult is one machine-readable eval outcome.
type CaseResult struct {
	ID         string   `json:"id"`
	Passed     bool     `json:"passed"`
	FinalState string   `json:"final_state"`
	Round      int      `json:"round"`
	FindingIDs []string `json:"finding_ids"`
	Operations []string `json:"operations_seen"`
	Failures   []string `json:"failures"`
	AuditKinds []string `json:"audit_kinds"`
}

// Report is the aggregate machine-readable harness output.
type Report struct {
	SchemaVersion string       `json:"schema_version"`
	GeneratedAt   time.Time    `json:"generated_at"`
	Passed        int          `json:"passed"`
	Failed        int          `json:"failed"`
	Results       []CaseResult `json:"results"`
}

// RunFixtures executes every fixture with the deterministic mock provider.
func RunFixtures(ctx context.Context, fixtures []Fixture, now time.Time) Report {
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		GeneratedAt:   now.UTC(),
		Results:       make([]CaseResult, 0, len(fixtures)),
	}
	for _, fixture := range fixtures {
		result := runOne(ctx, fixture, now)
		if result.Passed {
			report.Passed++
		} else {
			report.Failed++
		}
		report.Results = append(report.Results, result)
	}
	return report
}

func runOne(ctx context.Context, fixture Fixture, now time.Time) CaseResult {
	outcome := CaseResult{ID: fixture.ID, Failures: []string{}}
	provider := NewMockProvider(fixture.ProviderRounds)
	collector := NewCatalogCollector(fixture.EvidenceCatalog, now)
	loop := agentloop.Loop{
		Provider:  provider,
		Collector: collector,
		Options: agentloop.Options{
			Now:     func() time.Time { return now },
			Timeout: time.Minute,
		},
	}
	result, err := loop.Run(ctx, fixture.Report, fixture.Session)
	if err != nil && result.State == "" {
		outcome.Failures = append(outcome.Failures, err.Error())
		return outcome
	}
	outcome.FinalState = result.State
	outcome.Round = result.Round
	outcome.FindingIDs = findingIDs(result.Assessment)
	outcome.Operations = operationsSeen(result, provider.RequestedOperations())
	outcome.AuditKinds = auditKinds(result.AuditTimeline)

	expect := fixture.Expected
	if result.State != expect.FinalState {
		outcome.Failures = append(outcome.Failures, fmt.Sprintf("final_state=%q want %q", result.State, expect.FinalState))
	}
	if err != nil && expect.FinalState != agentloop.StateFailed && expect.FinalState != agentloop.StateBlocked {
		outcome.Failures = append(outcome.Failures, err.Error())
	}
	for _, findingID := range expect.FindingIDs {
		if !contains(outcome.FindingIDs, findingID) {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("missing finding %q", findingID))
		}
	}
	for _, ref := range expect.RequiredEvidenceRefs {
		if result.Assessment == nil || !assessmentHasEvidenceRef(*result.Assessment, ref) {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("missing required evidence ref %q", ref))
		}
	}
	allowed := map[string]struct{}{}
	for _, operation := range expect.AllowedOperationIDs {
		allowed[operation] = struct{}{}
	}
	for _, operation := range outcome.Operations {
		if _, ok := allowed[operation]; !ok && len(allowed) > 0 {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("operation %q is not allowed by the fixture", operation))
		}
		if !agentloop.IsAllowedEvidenceOperation(operation) && !isTechnicianNextStep(operation) {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("operation %q is outside the closed allowlist", operation))
		}
	}
	blob, _ := json.Marshal(result)
	text := strings.ToLower(string(blob))
	for _, claim := range expect.ForbiddenClaims {
		if claim != "" && strings.Contains(text, strings.ToLower(claim)) {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("forbidden claim %q appeared in the result", claim))
		}
	}
	outcome.Passed = len(outcome.Failures) == 0
	return outcome
}

func findingIDs(assessment *diagnosis.Assessment) []string {
	if assessment == nil {
		return []string{}
	}
	ids := make([]string, 0, len(assessment.Findings))
	for _, finding := range assessment.Findings {
		ids = append(ids, finding.ID)
	}
	sort.Strings(ids)
	return ids
}

func operationsSeen(result agentloop.Result, requested []string) []string {
	seen := map[string]struct{}{}
	for _, operation := range requested {
		seen[operation] = struct{}{}
	}
	if result.Assessment != nil {
		for _, step := range result.Assessment.NextSteps {
			seen[step.Operation] = struct{}{}
		}
	}
	for _, request := range result.EvidenceRequests {
		seen[request.Operation] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for operation := range seen {
		out = append(out, operation)
	}
	sort.Strings(out)
	return out
}

func auditKinds(events []agentloop.AuditEvent) []string {
	kinds := make([]string, 0, len(events))
	for _, event := range events {
		kinds = append(kinds, event.Kind)
	}
	return kinds
}

func assessmentHasEvidenceRef(assessment diagnosis.Assessment, ref string) bool {
	for _, finding := range assessment.Findings {
		for _, evidence := range finding.EvidenceRefs {
			if evidence == ref {
				return true
			}
		}
	}
	return false
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func isTechnicianNextStep(operation string) bool {
	switch operation {
	case agentloop.OpReviewMissingSources, agentloop.OpIdentifyWindowsInstallation, agentloop.OpSelectWindowsTarget,
		agentloop.OpInspectBCDEntries, agentloop.OpInspectStorageHealth, agentloop.OpReviewBitLockerAccess:
		return true
	default:
		return false
	}
}

// LoadFixtures reads every *.json fixture from directory.
func LoadFixtures(dir string) ([]Fixture, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fixtures := make([]Fixture, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var fixture Fixture
		if err := json.Unmarshal(data, &fixture); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if fixture.EvidenceCatalog == nil {
			fixture.EvidenceCatalog = map[string]agentloop.EvidencePayload{}
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures, nil
}

// WriteReport writes the machine-readable harness output.
func WriteReport(path string, report Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}
