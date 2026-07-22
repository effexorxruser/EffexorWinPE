package gateway

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

var readOnlyOperations = []string{
	"review_missing_sources",
	"identify_windows_installation",
	"select_windows_target",
	"inspect_bcd_entries",
	"inspect_storage_health",
	"review_bitlocker_access",
}

func ReadOnlyOperations() []string {
	return append([]string(nil), readOnlyOperations...)
}

func ValidateDiagnosisRequest(request DiagnosisRequest) error {
	if !request.TechnicianApproved {
		return fmt.Errorf("technician approval is required")
	}
	if request.DiagnosticReport.SchemaVersion != diagnostics.SchemaVersion {
		return fmt.Errorf("unsupported diagnostic schema %q", request.DiagnosticReport.SchemaVersion)
	}
	if strings.TrimSpace(request.DiagnosticReport.ReportID) == "" {
		return fmt.Errorf("report_id is required")
	}
	if err := request.Session.Validate(request.DiagnosticReport.ReportID); err != nil {
		return err
	}
	report := request.DiagnosticReport
	if len(report.Hardware.NetworkAdapters) > 128 || len(report.Storage.Disks) > 64 || len(report.Storage.DriveHealth) > 64 ||
		len(report.Storage.Partitions) > 512 || len(report.Storage.BitLockerVolumes) > 128 || len(report.Boot.BCDStores) > 64 ||
		len(report.Installations) > 32 || len(report.Checks) > 256 || len(report.Privacy.ExcludedByDefault) > 64 {
		return fmt.Errorf("diagnostic report exceeds bounded collection counts")
	}
	if len(request.Session.Symptoms) > 32 || len(request.Session.Answers) > 64 || len(request.Session.Events) > 256 {
		return fmt.Errorf("diagnostic session exceeds bounded context limits")
	}
	for _, symptom := range request.Session.Symptoms {
		if err := boundedText("symptom", symptom.Text, 2000); err != nil {
			return err
		}
	}
	for _, answer := range request.Session.Answers {
		if err := boundedText("answer", answer.Value, 2000); err != nil {
			return err
		}
	}
	if latest := request.Session.LatestAssessment; latest != nil && latest.ReportID != request.DiagnosticReport.ReportID {
		return fmt.Errorf("session assessment belongs to a different report")
	}
	return nil
}

// SanitizeDiagnosisRequest removes fields that are never needed by the model,
// even when a technician explicitly approves the remaining session upload.
func SanitizeDiagnosisRequest(request DiagnosisRequest) DiagnosisRequest {
	request.DiagnosticReport.Environment.Hostname = ""
	request.DiagnosticReport.Privacy.ContainsPersonalData = false
	request.Session.LatestAssessment = nil
	return request
}

func EvidenceReferences(request DiagnosisRequest) ([]string, error) {
	request = SanitizeDiagnosisRequest(request)
	result := map[string]struct{}{}
	if err := addJSONPaths("", request.DiagnosticReport, result); err != nil {
		return nil, err
	}
	technicianContext := map[string]any{
		"symptoms": request.Session.Symptoms,
		"answers":  request.Session.Answers,
	}
	if err := addJSONPaths("session", technicianContext, result); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(result))
	for path := range result {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths, nil
}

func addJSONPaths(prefix string, value any, result map[string]struct{}) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	var decoded any
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	collectJSONPaths(prefix, decoded, result)
	return nil
}

func collectJSONPaths(prefix string, value any, result map[string]struct{}) {
	if prefix != "" {
		result[prefix] = struct{}{}
	}
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			collectJSONPaths(path, child, result)
		}
	case []any:
		for index, child := range typed {
			collectJSONPaths(fmt.Sprintf("%s[%d]", prefix, index), child, result)
		}
	}
}

func ValidateOnlineAssessment(assessment diagnosis.Assessment, request DiagnosisRequest) error {
	if assessment.SchemaVersion != diagnosis.SchemaVersion {
		return fmt.Errorf("unsupported diagnosis schema %q", assessment.SchemaVersion)
	}
	if assessment.ReportID != request.DiagnosticReport.ReportID {
		return fmt.Errorf("assessment report_id does not match the request")
	}
	if assessment.Mode != diagnosis.ModeOnlineAgent {
		return fmt.Errorf("assessment mode must be %q", diagnosis.ModeOnlineAgent)
	}
	if assessment.GeneratedAt.IsZero() {
		return fmt.Errorf("assessment generated_at is required")
	}
	if err := boundedText("summary headline", assessment.Summary.Headline, 500); err != nil {
		return err
	}
	if !oneOf(assessment.Summary.HighestSeverity, diagnosis.SeverityInfo, diagnosis.SeverityWarning, diagnosis.SeverityCritical, diagnosis.SeverityUnknown) {
		return fmt.Errorf("summary contains an invalid severity")
	}
	if assessment.Summary.FindingCount != len(assessment.Findings) {
		return fmt.Errorf("summary finding_count does not match findings")
	}
	if assessment.Summary.HighestSeverity != highestSeverity(assessment.Findings) {
		return fmt.Errorf("summary highest_severity does not match findings")
	}
	if len(assessment.Findings) > 32 || len(assessment.Questions) > 16 || len(assessment.NextSteps) > 16 || len(assessment.Limitations) > 16 || len(assessment.Sources) > 64 {
		return fmt.Errorf("assessment exceeds bounded result limits")
	}

	validEvidence, err := EvidenceReferences(request)
	if err != nil {
		return fmt.Errorf("build evidence catalog: %w", err)
	}
	evidenceSet := make(map[string]struct{}, len(validEvidence))
	for _, reference := range validEvidence {
		evidenceSet[reference] = struct{}{}
	}
	sourceSet := map[string]struct{}{}
	for _, source := range assessment.Sources {
		if err := validateSource(source); err != nil {
			return err
		}
		if _, duplicate := sourceSet[source.URL]; duplicate {
			return fmt.Errorf("assessment contains a duplicate source URL")
		}
		sourceSet[source.URL] = struct{}{}
	}

	findingIDs := map[string]struct{}{}
	for _, finding := range assessment.Findings {
		if err := validateID("finding", finding.ID); err != nil {
			return err
		}
		if _, duplicate := findingIDs[finding.ID]; duplicate {
			return fmt.Errorf("duplicate finding id %q", finding.ID)
		}
		findingIDs[finding.ID] = struct{}{}
		if err := boundedText("finding title", finding.Title, 500); err != nil {
			return err
		}
		if err := boundedText("finding rationale", finding.Rationale, 4000); err != nil {
			return err
		}
		if !oneOf(finding.Severity, diagnosis.SeverityInfo, diagnosis.SeverityWarning, diagnosis.SeverityCritical, diagnosis.SeverityUnknown) {
			return fmt.Errorf("finding %q contains an invalid severity", finding.ID)
		}
		if !oneOf(finding.Confidence, diagnosis.ConfidenceLow, diagnosis.ConfidenceMedium, diagnosis.ConfidenceHigh) {
			return fmt.Errorf("finding %q contains invalid confidence", finding.ID)
		}
		if len(finding.EvidenceRefs) == 0 || len(finding.EvidenceRefs) > 16 || len(finding.SourceRefs) > 16 {
			return fmt.Errorf("finding %q contains invalid evidence or source counts", finding.ID)
		}
		if err := validateReferences("evidence", finding.ID, finding.EvidenceRefs, evidenceSet); err != nil {
			return err
		}
		if err := validateReferences("source", finding.ID, finding.SourceRefs, sourceSet); err != nil {
			return err
		}
	}

	questionIDs := map[string]struct{}{}
	for _, question := range assessment.Questions {
		if err := validateID("question", question.ID); err != nil {
			return err
		}
		if _, duplicate := questionIDs[question.ID]; duplicate {
			return fmt.Errorf("duplicate question id %q", question.ID)
		}
		questionIDs[question.ID] = struct{}{}
		if request.Session.HasAnswer(question.ID) {
			return fmt.Errorf("question %q was already answered in this session", question.ID)
		}
		if err := boundedText("question prompt", question.Prompt, 1000); err != nil {
			return err
		}
		if err := boundedText("question reason", question.Reason, 2000); err != nil {
			return err
		}
		if !oneOf(question.AnswerType, diagnosis.AnswerYesNo, diagnosis.AnswerFreeText) {
			return fmt.Errorf("question %q contains an invalid answer type", question.ID)
		}
	}

	operationSet := map[string]struct{}{}
	for _, operation := range readOnlyOperations {
		operationSet[operation] = struct{}{}
	}
	stepIDs := map[string]struct{}{}
	for _, step := range assessment.NextSteps {
		if err := validateID("next step", step.ID); err != nil {
			return err
		}
		if _, duplicate := stepIDs[step.ID]; duplicate {
			return fmt.Errorf("duplicate next-step id %q", step.ID)
		}
		stepIDs[step.ID] = struct{}{}
		if _, allowed := operationSet[step.Operation]; !allowed {
			return fmt.Errorf("next step %q contains unknown operation %q", step.ID, step.Operation)
		}
		if step.Risk != diagnosis.RiskReadOnly || step.RequiresConfirmation {
			return fmt.Errorf("online MVP may return read-only operations only")
		}
		if err := boundedText("next-step title", step.Title, 500); err != nil {
			return err
		}
		if err := boundedText("next-step rationale", step.Rationale, 2000); err != nil {
			return err
		}
	}
	if len(assessment.Limitations) == 0 {
		return fmt.Errorf("assessment must describe at least one limitation")
	}
	for _, limitation := range assessment.Limitations {
		if err := boundedText("limitation", limitation, 2000); err != nil {
			return err
		}
	}
	return nil
}

func validateSource(source diagnosis.Source) error {
	if err := boundedText("source title", source.Title, 1000); err != nil {
		return err
	}
	if len(source.URL) > 4096 {
		return fmt.Errorf("source URL is too long")
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
		return fmt.Errorf("source URL must be an absolute HTTPS URL")
	}
	domain := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if domain != strings.ToLower(strings.TrimSuffix(source.Domain, ".")) {
		return fmt.Errorf("source domain does not match its URL")
	}
	return nil
}

func validateReferences(kind, findingID string, values []string, allowed map[string]struct{}) error {
	seen := map[string]struct{}{}
	for _, value := range values {
		if _, duplicate := seen[value]; duplicate {
			return fmt.Errorf("finding %q repeats %s reference %q", findingID, kind, value)
		}
		seen[value] = struct{}{}
		if _, exists := allowed[value]; !exists {
			return fmt.Errorf("finding %q references unknown %s %q", findingID, kind, value)
		}
	}
	return nil
}

func boundedText(name, value string, maximum int) error {
	length := len([]rune(strings.TrimSpace(value)))
	if length == 0 || length > maximum {
		return fmt.Errorf("%s must contain between 1 and %d characters", name, maximum)
	}
	return nil
}

func validateID(name, value string) error {
	if value == "" || len(value) > 128 {
		return fmt.Errorf("%s id is empty or too long", name)
	}
	for _, character := range value {
		if character >= 'a' && character <= 'z' || character >= '0' && character <= '9' || character == '.' || character == '-' || character == '_' {
			continue
		}
		return fmt.Errorf("%s id %q contains unsupported characters", name, value)
	}
	return nil
}

func highestSeverity(findings []diagnosis.Finding) string {
	result := diagnosis.SeverityInfo
	rank := map[string]int{
		diagnosis.SeverityInfo:     0,
		diagnosis.SeverityUnknown:  1,
		diagnosis.SeverityWarning:  2,
		diagnosis.SeverityCritical: 3,
	}
	for _, finding := range findings {
		if rank[finding.Severity] > rank[result] {
			result = finding.Severity
		}
	}
	return result
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
