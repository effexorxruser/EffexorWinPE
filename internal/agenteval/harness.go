package agenteval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
	"github.com/santhosh-tekuri/jsonschema/v5"
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
	FinalRound           int      `json:"final_round"`
	FindingIDs           []string `json:"finding_ids"`
	ForbiddenClaims      []string `json:"forbidden_claims"`
	RequiredEvidenceRefs []string `json:"required_evidence_refs"`
	AllowedOperationIDs  []string `json:"allowed_operation_ids"`
	FailureCode          string   `json:"failure_code,omitempty"`
	BlockCode            string   `json:"block_code,omitempty"`
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
	Harness       string       `json:"harness"`
	Passed        int          `json:"passed"`
	Failed        int          `json:"failed"`
	Results       []CaseResult `json:"results"`
}

// RunFixtures executes every fixture with the deterministic mock provider.
func RunFixtures(ctx context.Context, fixtures []Fixture, now time.Time) Report {
	return runFixturesNamed(ctx, fixtures, now, "scenario-eval")
}

// RunPolicyRegressionFixtures executes policy-focused fixtures under a distinct harness name.
func RunPolicyRegressionFixtures(ctx context.Context, fixtures []Fixture, now time.Time) Report {
	return runFixturesNamed(ctx, fixtures, now, "policy-regression")
}

func runFixturesNamed(ctx context.Context, fixtures []Fixture, now time.Time, harness string) Report {
	report := Report{
		SchemaVersion: ReportSchemaVersion,
		GeneratedAt:   now.UTC(),
		Harness:       harness,
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
	if err := validateFixtureShape(fixture); err != nil {
		outcome.Failures = append(outcome.Failures, err.Error())
		return outcome
	}
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
	if result.Round != expect.FinalRound {
		outcome.Failures = append(outcome.Failures, fmt.Sprintf("final_round=%d want %d", result.Round, expect.FinalRound))
	}
	if err != nil && expect.FinalState != agentloop.StateFailed && expect.FinalState != agentloop.StateBlocked {
		outcome.Failures = append(outcome.Failures, err.Error())
	}
	if expect.FailureCode != "" {
		if result.Failure == nil || result.Failure.Code != expect.FailureCode {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("failure_code=%v want %q", result.Failure, expect.FailureCode))
		}
	}
	if expect.BlockCode != "" {
		if result.Block == nil || result.Block.Code != expect.BlockCode {
			outcome.Failures = append(outcome.Failures, fmt.Sprintf("block_code=%v want %q", result.Block, expect.BlockCode))
		}
	}
	expectedFindings := append([]string{}, expect.FindingIDs...)
	sort.Strings(expectedFindings)
	actualFindings := append([]string{}, outcome.FindingIDs...)
	sort.Strings(actualFindings)
	if strings.Join(actualFindings, "\n") != strings.Join(expectedFindings, "\n") {
		outcome.Failures = append(outcome.Failures, fmt.Sprintf("finding_ids=%v want exact %v", actualFindings, expectedFindings))
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

func validateFixtureShape(fixture Fixture) error {
	if strings.TrimSpace(fixture.ID) == "" {
		return fmt.Errorf("fixture id is required")
	}
	if fixture.Expected.FinalRound < 1 || fixture.Expected.FinalRound > agentloop.MaxRounds {
		return fmt.Errorf("fixture %s expected.final_round is invalid", fixture.ID)
	}
	if fixture.Expected.FindingIDs == nil || fixture.Expected.ForbiddenClaims == nil ||
		fixture.Expected.RequiredEvidenceRefs == nil || fixture.Expected.AllowedOperationIDs == nil {
		return fmt.Errorf("fixture %s expected arrays must be present", fixture.ID)
	}
	if fixture.EvidenceCatalog == nil {
		return fmt.Errorf("fixture %s evidence_catalog must be present", fixture.ID)
	}
	if len(fixture.ProviderRounds) == 0 {
		return fmt.Errorf("fixture %s provider_rounds must be present", fixture.ID)
	}
	return nil
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

func isTechnicianNextStep(operation string) bool {
	switch operation {
	case agentloop.OpReviewMissingSources, agentloop.OpIdentifyWindowsInstallation, agentloop.OpSelectWindowsTarget,
		agentloop.OpInspectBCDEntries, agentloop.OpInspectStorageHealth, agentloop.OpReviewBitLockerAccess:
		return true
	default:
		return false
	}
}

// LoadFixtures reads every *.json fixture from directory with strict decoding.
func LoadFixtures(dir string) ([]Fixture, error) {
	schema, err := compileFixtureSchema()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fixtures := make([]Fixture, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var raw any
		if err := json.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if err := schema.Validate(raw); err != nil {
			return nil, fmt.Errorf("%s fixture schema: %w", entry.Name(), err)
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var fixture Fixture
		if err := decoder.Decode(&fixture); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		var extra any
		if err := decoder.Decode(&extra); err != io.EOF {
			if err == nil {
				return nil, fmt.Errorf("%s: trailing JSON data", entry.Name())
			}
			return nil, fmt.Errorf("%s: trailing JSON: %w", entry.Name(), err)
		}
		if fixture.EvidenceCatalog == nil {
			fixture.EvidenceCatalog = map[string]agentloop.EvidencePayload{}
		}
		if err := validateFixtureShape(fixture); err != nil {
			return nil, err
		}
		fixtures = append(fixtures, fixture)
	}
	sort.Slice(fixtures, func(i, j int) bool { return fixtures[i].ID < fixtures[j].ID })
	return fixtures, nil
}

func compileFixtureSchema() (*jsonschema.Schema, error) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "contracts", "agent-eval-fixture.schema.json"))
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource("https://effexorwinpe.local/contracts/agent-eval-fixture.schema.json", bytes.NewReader(raw)); err != nil {
		return nil, err
	}
	return compiler.Compile("https://effexorwinpe.local/contracts/agent-eval-fixture.schema.json")
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
