package agentloop

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
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

// ValidationContext binds a proposal to the active report, session, prior
// evidence, and provider-issued source URLs.
type ValidationContext struct {
	Report             diagnostics.Report
	Session            session.Session
	PriorEvidence      []EvidencePayload
	PriorRequestKeys   []string
	MaxRounds          int
	ProviderSourceURLs []string
}

// ValidateProposal enforces the strict provider contract. Required fields are
// not invented by the loop.
func ValidateProposal(proposal ProviderProposal, ctx ValidationContext) error {
	if err := ctx.Session.Validate(ctx.Report.ReportID); err != nil {
		return fmt.Errorf("session context: %w", err)
	}
	if ctx.Report.SchemaVersion != diagnostics.SchemaVersion {
		return fmt.Errorf("unsupported diagnostic schema %q", ctx.Report.SchemaVersion)
	}
	if ctx.Report.ReportID == "" {
		return fmt.Errorf("report_id is required")
	}
	if proposal.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported agent-result schema %q", proposal.SchemaVersion)
	}
	if proposal.ReportID != ctx.Report.ReportID {
		return fmt.Errorf("proposal report_id %q does not match report %q", proposal.ReportID, ctx.Report.ReportID)
	}
	if proposal.GeneratedAt.IsZero() {
		return fmt.Errorf("generated_at is required")
	}
	maxRounds := ctx.MaxRounds
	if maxRounds <= 0 || maxRounds > MaxRounds {
		maxRounds = MaxRounds
	}
	if proposal.Round < 1 || proposal.Round > maxRounds {
		return fmt.Errorf("round must be between 1 and %d", maxRounds)
	}
	if !oneOf(proposal.State, StateCompleted, StateNeedsMoreEvidence, StateBlocked, StateFailed) {
		return fmt.Errorf("invalid agent state %q", proposal.State)
	}
	if proposal.EvidenceRequests == nil {
		return fmt.Errorf("evidence_requests must be present")
	}
	if proposal.RetrievedSources == nil {
		return fmt.Errorf("retrieved_sources must be present")
	}
	if len(proposal.EvidenceRequests) > MaxEvidenceRequests {
		return fmt.Errorf("evidence_requests exceeds %d entries", MaxEvidenceRequests)
	}
	if len(proposal.Limitations) == 0 || len(proposal.Limitations) > MaxLimitations {
		return fmt.Errorf("limitations must contain between 1 and %d entries", MaxLimitations)
	}
	for _, limitation := range proposal.Limitations {
		if err := boundedText("limitation", limitation, 2000); err != nil {
			return err
		}
		if err := RejectCommandText("limitation", limitation); err != nil {
			return err
		}
	}
	if err := validateSize("provider proposal", proposal, MaxResponseBytes); err != nil {
		return err
	}
	providerURLs, err := validateRetrievedSources(proposal.RetrievedSources)
	if err != nil {
		return err
	}
	if len(ctx.ProviderSourceURLs) > 0 {
		// Prefer explicitly supplied context URLs when present.
		providerURLs = append([]string{}, ctx.ProviderSourceURLs...)
	}

	seenIDs := map[string]struct{}{}
	seenKeys := map[string]struct{}{}
	for _, prior := range ctx.PriorRequestKeys {
		seenKeys[prior] = struct{}{}
	}
	for index := range proposal.EvidenceRequests {
		request := &proposal.EvidenceRequests[index]
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
		key := CanonicalRequestKey(*request)
		if _, duplicate := seenKeys[key]; duplicate {
			return &DuplicateEvidenceError{RequestID: request.ID, Key: key}
		}
		seenKeys[key] = struct{}{}
	}

	switch proposal.State {
	case StateCompleted:
		if proposal.Assessment == nil {
			return fmt.Errorf("completed result requires assessment")
		}
		if len(proposal.EvidenceRequests) != 0 {
			return fmt.Errorf("completed result must not request more evidence")
		}
		if proposal.Block != nil || proposal.Failure != nil {
			return fmt.Errorf("completed result must not include block or failure details")
		}
		if proposal.Assessment.ReportID != ctx.Report.ReportID {
			return fmt.Errorf("assessment report_id %q does not match report %q", proposal.Assessment.ReportID, ctx.Report.ReportID)
		}
		if proposal.Assessment.Mode != diagnosis.ModeOnlineAgent {
			return fmt.Errorf("assessment mode must be %q", diagnosis.ModeOnlineAgent)
		}
		if proposal.Assessment.SchemaVersion != diagnosis.SchemaVersion {
			return fmt.Errorf("unsupported diagnosis schema %q", proposal.Assessment.SchemaVersion)
		}
		if proposal.Assessment.GeneratedAt.IsZero() {
			return fmt.Errorf("assessment generated_at is required")
		}
		if err := rejectAssessmentCommandText(*proposal.Assessment); err != nil {
			return err
		}
		if err := validateAssessmentSourcesAgainstProvider(*proposal.Assessment, providerURLs); err != nil {
			return err
		}
		diagnosisRequest := gateway.DiagnosisRequest{
			DiagnosticReport:   ctx.Report,
			Session:            ctx.Session,
			TechnicianApproved: true,
		}
		if err := gateway.ValidateOnlineAssessmentWithEvidence(*proposal.Assessment, diagnosisRequest, collectedEvidenceRefs(ctx.PriorEvidence)); err != nil {
			return fmt.Errorf("gateway assessment validation: %w", err)
		}
	case StateNeedsMoreEvidence:
		if len(proposal.EvidenceRequests) == 0 {
			return fmt.Errorf("needs_more_evidence requires at least one evidence request")
		}
		if proposal.Assessment != nil {
			return fmt.Errorf("needs_more_evidence must not include a final assessment")
		}
		if proposal.Block != nil || proposal.Failure != nil {
			return fmt.Errorf("needs_more_evidence must not include block or failure details")
		}
	case StateBlocked:
		if proposal.Block == nil {
			return fmt.Errorf("blocked result requires block details")
		}
		if err := validateStatusDetail("block", *proposal.Block); err != nil {
			return err
		}
		if len(proposal.EvidenceRequests) != 0 {
			return fmt.Errorf("blocked result must not request more evidence")
		}
		if proposal.Failure != nil {
			return fmt.Errorf("blocked result must not include failure details")
		}
		if proposal.Assessment != nil {
			return fmt.Errorf("blocked result must not include an assessment")
		}
	case StateFailed:
		if proposal.Failure == nil {
			return fmt.Errorf("failed result requires failure details")
		}
		if err := validateStatusDetail("failure", *proposal.Failure); err != nil {
			return err
		}
		if len(proposal.EvidenceRequests) != 0 {
			return fmt.Errorf("failed result must not request more evidence")
		}
		if proposal.Block != nil {
			return fmt.Errorf("failed result must not include block details")
		}
		if proposal.Assessment != nil {
			return fmt.Errorf("failed result must not include an assessment")
		}
	}
	return nil
}

func validateRetrievedSources(sources []diagnosis.Source) ([]string, error) {
	urls := make([]string, 0, len(sources))
	seen := map[string]struct{}{}
	for _, source := range sources {
		if err := boundedText("retrieved source title", source.Title, 1000); err != nil {
			return nil, err
		}
		parsed, err := url.Parse(strings.TrimSpace(source.URL))
		if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil {
			return nil, fmt.Errorf("retrieved source URL must be absolute HTTPS")
		}
		domain := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
		if domain != strings.ToLower(strings.TrimSuffix(source.Domain, ".")) {
			return nil, fmt.Errorf("retrieved source domain does not match its URL")
		}
		if _, duplicate := seen[source.URL]; duplicate {
			return nil, fmt.Errorf("duplicate retrieved source URL")
		}
		seen[source.URL] = struct{}{}
		urls = append(urls, source.URL)
	}
	return urls, nil
}

func validateAssessmentSourcesAgainstProvider(assessment diagnosis.Assessment, providerURLs []string) error {
	allowed := map[string]struct{}{}
	for _, raw := range providerURLs {
		allowed[raw] = struct{}{}
	}
	for _, source := range assessment.Sources {
		if _, ok := allowed[source.URL]; !ok {
			return fmt.Errorf("assessment source URL %q was not issued by the provider API", source.URL)
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
