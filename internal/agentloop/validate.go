package agentloop

import (
	"encoding/json"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
)

const (
	MaxRounds           = 3
	MaxRequestBytes     = 1 << 20
	MaxResponseBytes    = 1 << 20
	MaxEvidenceRequests = 8
	MaxAuditEvents      = 64
	MaxLimitations      = 16
	DefaultLoopTimeout  = 90 // seconds, documented default for callers
)

func ValidateResult(result Result, priorRequestKeys []string) error {
	if result.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported agent-result schema %q", result.SchemaVersion)
	}
	if strings.TrimSpace(result.ReportID) == "" {
		return fmt.Errorf("report_id is required")
	}
	if result.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	if result.Round < 1 || result.Round > MaxRounds {
		return fmt.Errorf("round must be between 1 and %d", MaxRounds)
	}
	if !oneOf(result.State, StateCompleted, StateNeedsMoreEvidence, StateBlocked, StateFailed) {
		return fmt.Errorf("invalid agent state %q", result.State)
	}
	if result.EvidenceRequests == nil {
		return fmt.Errorf("evidence_requests must be present")
	}
	if len(result.EvidenceRequests) > MaxEvidenceRequests {
		return fmt.Errorf("evidence_requests exceeds %d entries", MaxEvidenceRequests)
	}
	if len(result.AuditTimeline) > MaxAuditEvents {
		return fmt.Errorf("audit_timeline exceeds %d entries", MaxAuditEvents)
	}
	if len(result.Limitations) == 0 || len(result.Limitations) > MaxLimitations {
		return fmt.Errorf("limitations must contain between 1 and %d entries", MaxLimitations)
	}
	for _, limitation := range result.Limitations {
		if err := boundedText("limitation", limitation, 2000); err != nil {
			return err
		}
		if err := RejectCommandText("limitation", limitation); err != nil {
			return err
		}
	}
	if err := validateSize("agent result", result, MaxResponseBytes); err != nil {
		return err
	}

	seenIDs := map[string]struct{}{}
	seenKeys := map[string]struct{}{}
	for _, prior := range priorRequestKeys {
		seenKeys[prior] = struct{}{}
	}
	for _, request := range result.EvidenceRequests {
		if err := ValidateEvidenceRequest(request); err != nil {
			return err
		}
		if err := RejectCommandText("evidence reason", request.Reason); err != nil {
			return err
		}
		if err := RejectCommandText("expected information", request.ExpectedInformation); err != nil {
			return err
		}
		if _, duplicate := seenIDs[request.ID]; duplicate {
			return fmt.Errorf("duplicate evidence request id %q", request.ID)
		}
		seenIDs[request.ID] = struct{}{}
		key := CanonicalRequestKey(request)
		if _, duplicate := seenKeys[key]; duplicate {
			return fmt.Errorf("duplicate evidence request %q", key)
		}
		seenKeys[key] = struct{}{}
	}

	switch result.State {
	case StateCompleted:
		if result.Assessment == nil {
			return fmt.Errorf("completed result requires assessment")
		}
		if len(result.EvidenceRequests) != 0 {
			return fmt.Errorf("completed result must not request more evidence")
		}
		if result.Block != nil || result.Failure != nil {
			return fmt.Errorf("completed result must not include block or failure details")
		}
		if err := validateAssessmentText(*result.Assessment); err != nil {
			return err
		}
	case StateNeedsMoreEvidence:
		if len(result.EvidenceRequests) == 0 {
			return fmt.Errorf("needs_more_evidence requires at least one evidence request")
		}
		if result.Assessment != nil {
			return fmt.Errorf("needs_more_evidence must not include a final assessment")
		}
		if result.Block != nil || result.Failure != nil {
			return fmt.Errorf("needs_more_evidence must not include block or failure details")
		}
	case StateBlocked:
		if result.Block == nil {
			return fmt.Errorf("blocked result requires block details")
		}
		if err := validateStatusDetail("block", *result.Block); err != nil {
			return err
		}
		if result.Failure != nil {
			return fmt.Errorf("blocked result must not include failure details")
		}
		if result.Assessment != nil {
			return fmt.Errorf("blocked result must not include an assessment")
		}
	case StateFailed:
		if result.Failure == nil {
			return fmt.Errorf("failed result requires failure details")
		}
		if err := validateStatusDetail("failure", *result.Failure); err != nil {
			return err
		}
		if result.Block != nil {
			return fmt.Errorf("failed result must not include block details")
		}
		if result.Assessment != nil {
			return fmt.Errorf("failed result must not include an assessment")
		}
	}
	return nil
}

func validateAssessmentText(assessment diagnosis.Assessment) error {
	if err := RejectCommandText("headline", assessment.Summary.Headline); err != nil {
		return err
	}
	for _, finding := range assessment.Findings {
		if err := RejectCommandText("finding title", finding.Title); err != nil {
			return err
		}
		if err := RejectCommandText("finding rationale", finding.Rationale); err != nil {
			return err
		}
	}
	for _, question := range assessment.Questions {
		if err := RejectCommandText("question prompt", question.Prompt); err != nil {
			return err
		}
		if err := RejectCommandText("question reason", question.Reason); err != nil {
			return err
		}
	}
	for _, step := range assessment.NextSteps {
		if err := RejectCommandText("next-step title", step.Title); err != nil {
			return err
		}
		if err := RejectCommandText("next-step rationale", step.Rationale); err != nil {
			return err
		}
		if !isTechnicianNextStep(step.Operation) {
			return fmt.Errorf("next step %q uses unknown operation %q", step.ID, step.Operation)
		}
		if step.Risk != diagnosis.RiskReadOnly || step.RequiresConfirmation {
			return fmt.Errorf("agent loop may propose read-only next steps only")
		}
	}
	for _, limitation := range assessment.Limitations {
		if err := RejectCommandText("assessment limitation", limitation); err != nil {
			return err
		}
	}
	return nil
}

func isTechnicianNextStep(operation string) bool {
	// Technician-facing next steps stay aligned with diagnosis.schema.json /
	// the existing gateway MVP catalog. Evidence requests may use a wider
	// read-only allowlist during the loop.
	switch operation {
	case OpReviewMissingSources, OpIdentifyWindowsInstallation, OpSelectWindowsTarget,
		OpInspectBCDEntries, OpInspectStorageHealth, OpReviewBitLockerAccess:
		return true
	default:
		return false
	}
}

func validateStatusDetail(name string, detail StatusDetail) error {
	if err := validateID(name+" code", detail.Code); err != nil {
		return err
	}
	if err := boundedText(name+" message", detail.Message, 2000); err != nil {
		return err
	}
	return RejectCommandText(name+" message", detail.Message)
}

func validateSize(name string, value any, maximum int) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", name, err)
	}
	if len(data) > maximum {
		return fmt.Errorf("%s exceeds %d bytes", name, maximum)
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

func boundedText(name, value string, maximum int) error {
	trimmed := strings.TrimSpace(value)
	length := utf8.RuneCountInString(trimmed)
	if length == 0 || length > maximum {
		return fmt.Errorf("%s must contain between 1 and %d characters", name, maximum)
	}
	return nil
}

func oneOf(value string, allowed ...string) bool {
	for _, candidate := range allowed {
		if value == candidate {
			return true
		}
	}
	return false
}
