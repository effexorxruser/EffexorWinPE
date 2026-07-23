package agentloop

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/gateway"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

const (
	MaxRounds           = 3
	MaxRequestBytes     = 1 << 20
	MaxResponseBytes    = 1 << 20
	MaxEvidenceRequests = 8
	MaxAuditEvents      = 64
	MaxLimitations      = 16
	MaxEvidencePayload  = 256 << 10
	DefaultLoopTimeout  = 90 // seconds, documented default for callers
)

// DuplicateEvidenceError marks a repeated evidence request fingerprint.
type DuplicateEvidenceError struct {
	RequestID string
	Key       string
}

func (err *DuplicateEvidenceError) Error() string {
	return fmt.Sprintf("duplicate evidence request %q", err.Key)
}

// ValidationContext binds a proposal to the active report, session, and prior evidence.
type ValidationContext struct {
	Report           diagnostics.Report
	Session          session.Session
	PriorEvidence    []EvidencePayload
	PriorRequestKeys []string
	MaxRounds        int
}

func ValidateResult(result Result, ctx ValidationContext) error {
	if err := ctx.Session.Validate(ctx.Report.ReportID); err != nil {
		return fmt.Errorf("session context: %w", err)
	}
	if ctx.Report.SchemaVersion != diagnostics.SchemaVersion {
		return fmt.Errorf("unsupported diagnostic schema %q", ctx.Report.SchemaVersion)
	}
	if ctx.Report.ReportID == "" {
		return fmt.Errorf("report_id is required")
	}
	if result.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported agent-result schema %q", result.SchemaVersion)
	}
	if result.ReportID != ctx.Report.ReportID {
		return fmt.Errorf("proposal report_id %q does not match report %q", result.ReportID, ctx.Report.ReportID)
	}
	if result.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	maxRounds := ctx.MaxRounds
	if maxRounds <= 0 || maxRounds > MaxRounds {
		maxRounds = MaxRounds
	}
	if result.Round < 1 || result.Round > maxRounds {
		return fmt.Errorf("round must be between 1 and %d", maxRounds)
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
	for _, prior := range ctx.PriorRequestKeys {
		seenKeys[prior] = struct{}{}
	}
	for _, request := range result.EvidenceRequests {
		if err := ValidateEvidenceRequest(request, ctx.Report); err != nil {
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
			return &DuplicateEvidenceError{RequestID: request.ID, Key: key}
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
		if result.Assessment.ReportID != ctx.Report.ReportID {
			return fmt.Errorf("assessment report_id %q does not match report %q", result.Assessment.ReportID, ctx.Report.ReportID)
		}
		if result.Assessment.Mode != diagnosis.ModeOnlineAgent {
			return fmt.Errorf("assessment mode must be %q", diagnosis.ModeOnlineAgent)
		}
		if result.Assessment.SchemaVersion != diagnosis.SchemaVersion {
			return fmt.Errorf("unsupported diagnosis schema %q", result.Assessment.SchemaVersion)
		}
		if result.Assessment.GeneratedAt.IsZero() {
			return fmt.Errorf("assessment generated_at is required")
		}
		if err := rejectAssessmentCommandText(*result.Assessment); err != nil {
			return err
		}
		diagnosisRequest := gateway.DiagnosisRequest{
			DiagnosticReport:   ctx.Report,
			Session:            ctx.Session,
			TechnicianApproved: true,
		}
		if err := gateway.ValidateOnlineAssessmentWithEvidence(*result.Assessment, diagnosisRequest, collectedEvidenceRefs(ctx.PriorEvidence)); err != nil {
			return fmt.Errorf("gateway assessment validation: %w", err)
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
		if len(result.EvidenceRequests) != 0 {
			return fmt.Errorf("blocked result must not request more evidence")
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
		if len(result.EvidenceRequests) != 0 {
			return fmt.Errorf("failed result must not request more evidence")
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

func collectedEvidenceRefs(payloads []EvidencePayload) []string {
	refs := make([]string, 0, len(payloads))
	seen := map[string]struct{}{}
	for _, payload := range payloads {
		for _, ref := range payload.EvidenceRefs {
			ref = strings.TrimSpace(ref)
			if ref == "" {
				continue
			}
			if _, ok := seen[ref]; ok {
				continue
			}
			seen[ref] = struct{}{}
			refs = append(refs, ref)
		}
	}
	return refs
}

func rejectAssessmentCommandText(assessment diagnosis.Assessment) error {
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
	}
	for _, limitation := range assessment.Limitations {
		if err := RejectCommandText("assessment limitation", limitation); err != nil {
			return err
		}
	}
	return nil
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

func IsDuplicateEvidence(err error) bool {
	var duplicate *DuplicateEvidenceError
	return errors.As(err, &duplicate)
}
