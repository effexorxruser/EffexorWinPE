package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

// RoundProvider proposes one agent-loop turn using only sanitized context.
type RoundProvider interface {
	Propose(ctx context.Context, input RoundInput) (ProviderProposal, error)
}

// EvidenceCollector fulfills validated read-only evidence requests locally.
type EvidenceCollector interface {
	Collect(ctx context.Context, request EvidenceRequest) (EvidencePayload, error)
}

// Options bounds the multi-step diagnostic loop.
type Options struct {
	MaxRounds        int
	Timeout          time.Duration
	MaxRequestBytes  int
	MaxResponseBytes int
	Now              func() time.Time
}

// Loop runs a bounded diagnostic agent conversation without executing model
// command text.
type Loop struct {
	Provider  RoundProvider
	Collector EvidenceCollector
	Options   Options
}

// Run executes at most Options.MaxRounds provider turns, collecting evidence
// between needs_more_evidence proposals.
func (loop Loop) Run(ctx context.Context, report diagnostics.Report, sess session.Session) (Result, error) {
	if loop.Provider == nil {
		return Result{}, fmt.Errorf("agent loop provider is required")
	}
	if err := sess.Validate(report.ReportID); err != nil {
		return Result{}, fmt.Errorf("session context: %w", err)
	}
	if report.SchemaVersion != diagnostics.SchemaVersion {
		return Result{}, fmt.Errorf("unsupported diagnostic schema %q", report.SchemaVersion)
	}
	options := loop.normalizedOptions()
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

	sanitized := NewSanitizedAgentContext(report, sess)
	audit := []AuditEvent{}
	priorEvidence := []EvidencePayload{}
	priorKeys := []string{}
	var last Result

	for round := 1; round <= options.MaxRounds; round++ {
		if err := ctx.Err(); err != nil {
			return loop.fail(report.ReportID, round, audit, "context_canceled", "Agent loop stopped before completion.", options.Now()), err
		}
		audit = append(audit, AuditEvent{
			At:    options.Now().UTC(),
			Kind:  AuditRoundStarted,
			Round: round,
		})

		input := RoundInput{
			Context:          sanitized,
			Round:            round,
			PriorEvidence:    FilterUploadableEvidence(priorEvidence),
			PriorRequestKeys: append([]string(nil), priorKeys...),
		}
		if err := validateSize("round input", input, options.MaxRequestBytes); err != nil {
			return loop.fail(report.ReportID, round, audit, "request_too_large", err.Error(), options.Now()), err
		}

		proposal, err := loop.Provider.Propose(ctx, input)
		if err != nil {
			return loop.fail(report.ReportID, round, audit, "provider_error", "Provider failed without changing the client system.", options.Now()), err
		}
		if err := validateSize("provider proposal", proposal, options.MaxResponseBytes); err != nil {
			return loop.fail(report.ReportID, round, audit, "response_too_large", err.Error(), options.Now()), err
		}

		providerURLs := make([]string, 0, len(proposal.RetrievedSources))
		for _, source := range proposal.RetrievedSources {
			providerURLs = append(providerURLs, source.URL)
		}
		validation := ValidationContext{
			Report:             report,
			Session:            sess,
			PriorEvidence:      FilterUploadableEvidence(priorEvidence),
			PriorRequestKeys:   priorKeys,
			MaxRounds:          options.MaxRounds,
			ProviderSourceURLs: providerURLs,
		}
		if err := ValidateProposal(proposal, validation); err != nil {
			if IsDuplicateEvidence(err) {
				audit = append(audit, AuditEvent{
					At:        options.Now().UTC(),
					Kind:      AuditDuplicateRequestRejected,
					Round:     round,
					Reference: proposal.ReportID,
					Detail:    err.Error(),
				})
			}
			return loop.fail(report.ReportID, round, audit, "invalid_provider_result", "Provider result failed policy validation.", options.Now()), err
		}
		audit = append(audit, AuditEvent{
			At:        options.Now().UTC(),
			Kind:      AuditProviderProposed,
			Round:     round,
			Reference: proposal.State,
		})
		result := proposal.toResult(audit)
		last = result

		switch proposal.State {
		case StateCompleted:
			audit = append(audit, AuditEvent{At: options.Now().UTC(), Kind: AuditLoopCompleted, Round: round})
			result.AuditTimeline = append([]AuditEvent{}, audit...)
			return result, nil
		case StateBlocked:
			audit = append(audit, AuditEvent{
				At:        options.Now().UTC(),
				Kind:      AuditLoopBlocked,
				Round:     round,
				Reference: proposal.Block.Code,
			})
			result.AuditTimeline = append([]AuditEvent{}, audit...)
			return result, nil
		case StateFailed:
			audit = append(audit, AuditEvent{
				At:        options.Now().UTC(),
				Kind:      AuditLoopFailed,
				Round:     round,
				Reference: proposal.Failure.Code,
			})
			result.AuditTimeline = append([]AuditEvent{}, audit...)
			return result, nil
		case StateNeedsMoreEvidence:
			if round == options.MaxRounds {
				blocked := Result{
					SchemaVersion:    SchemaVersion,
					ReportID:         report.ReportID,
					GeneratedAt:      options.Now().UTC(),
					State:            StateBlocked,
					Round:            round,
					EvidenceRequests: []EvidenceRequest{},
					Block: &StatusDetail{
						Code:    "max_rounds_exceeded",
						Message: fmt.Sprintf("Evidence gathering stopped after %d rounds.", options.MaxRounds),
					},
					Limitations: []string{
						fmt.Sprintf("The agent loop stopped after %d rounds without a completed assessment.", options.MaxRounds),
					},
				}
				audit = append(audit, AuditEvent{
					At:        options.Now().UTC(),
					Kind:      AuditLoopBlocked,
					Round:     round,
					Reference: "max_rounds_exceeded",
				})
				blocked.AuditTimeline = append([]AuditEvent{}, audit...)
				return blocked, nil
			}
			if loop.Collector == nil {
				return loop.fail(report.ReportID, round, audit, "collector_missing", "Evidence collector is not configured.", options.Now()), fmt.Errorf("evidence collector is required")
			}
			for _, request := range proposal.EvidenceRequests {
				key := CanonicalRequestKey(request)
				audit = append(audit, AuditEvent{
					At:        options.Now().UTC(),
					Kind:      AuditEvidenceRequested,
					Round:     round,
					Reference: request.ID,
					Detail:    key,
				})
				collectCtx, cancel := context.WithTimeout(ctx, time.Duration(request.TimeoutSeconds)*time.Second)
				payload, err := loop.Collector.Collect(collectCtx, request)
				cancel()
				if err != nil {
					return loop.fail(report.ReportID, round, audit, "evidence_collection_failed", "Read-only evidence collection failed.", options.Now()), err
				}
				normalized, err := NormalizeAndValidateEvidencePayload(payload, request)
				if err != nil {
					return loop.fail(report.ReportID, round, audit, "invalid_evidence_payload", "Collected evidence failed validation.", options.Now()), err
				}
				priorEvidence = append(priorEvidence, normalized)
				priorKeys = append(priorKeys, key)
				audit = append(audit, AuditEvent{
					At:        options.Now().UTC(),
					Kind:      AuditEvidenceCollected,
					Round:     round,
					Reference: normalized.RequestID,
				})
				if !MayUploadEvidence(normalized.PrivacyClass) {
					audit = append(audit, AuditEvent{
						At:        options.Now().UTC(),
						Kind:      AuditEvidenceRedacted,
						Round:     round,
						Reference: normalized.RequestID,
						Detail:    "privacy_class blocks upload to the next provider round",
					})
				}
			}
		default:
			return loop.fail(report.ReportID, round, audit, "invalid_state", "Provider returned an unsupported state.", options.Now()), fmt.Errorf("unsupported state %q", proposal.State)
		}
	}

	if last.ReportID == "" {
		return loop.fail(report.ReportID, options.MaxRounds, audit, "loop_exhausted", "Agent loop ended without a terminal result.", options.Now()), fmt.Errorf("agent loop exhausted")
	}
	return last, nil
}

func (loop Loop) normalizedOptions() Options {
	options := loop.Options
	if options.MaxRounds <= 0 || options.MaxRounds > MaxRounds {
		options.MaxRounds = MaxRounds
	}
	if options.Timeout <= 0 {
		options.Timeout = time.Duration(DefaultLoopTimeout) * time.Second
	}
	if options.MaxRequestBytes <= 0 {
		options.MaxRequestBytes = MaxRequestBytes
	}
	if options.MaxResponseBytes <= 0 {
		options.MaxResponseBytes = MaxResponseBytes
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	return options
}

func (loop Loop) fail(reportID string, round int, audit []AuditEvent, code, message string, now time.Time) Result {
	if len(audit) == 0 || audit[len(audit)-1].Kind != AuditLoopFailed {
		audit = append(audit, AuditEvent{
			At:        now.UTC(),
			Kind:      AuditLoopFailed,
			Round:     round,
			Reference: code,
			Detail:    message,
		})
	}
	return Result{
		SchemaVersion:    SchemaVersion,
		ReportID:         reportID,
		GeneratedAt:      now.UTC(),
		State:            StateFailed,
		Round:            round,
		EvidenceRequests: []EvidenceRequest{},
		Failure:          &StatusDetail{Code: code, Message: message},
		AuditTimeline:    append([]AuditEvent{}, audit...),
		Limitations:      []string{"The agent loop failed closed without mutating the client system."},
	}
}

// EncodeRoundInput marshals a round input for size checks and deterministic tests.
func EncodeRoundInput(input RoundInput) ([]byte, error) {
	return json.Marshal(input)
}
