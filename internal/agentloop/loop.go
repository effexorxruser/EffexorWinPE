package agentloop

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

// RoundProvider proposes one agent-loop turn. Implementations must remain
// provider-neutral; the existing gateway Analyzer interface stays unchanged.
type RoundProvider interface {
	Propose(ctx context.Context, input RoundInput) (Result, error)
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
	options := loop.normalizedOptions()
	if options.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, options.Timeout)
		defer cancel()
	}

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
			Report:           report,
			Session:          sess,
			Round:            round,
			PriorEvidence:    append([]EvidencePayload(nil), priorEvidence...),
			PriorRequestKeys: append([]string(nil), priorKeys...),
		}
		if err := validateSize("round input", input, options.MaxRequestBytes); err != nil {
			return loop.fail(report.ReportID, round, audit, "request_too_large", err.Error(), options.Now()), err
		}

		proposal, err := loop.Provider.Propose(ctx, input)
		if err != nil {
			audit = append(audit, AuditEvent{
				At:     options.Now().UTC(),
				Kind:   AuditLoopFailed,
				Round:  round,
				Detail: "provider error",
			})
			return loop.fail(report.ReportID, round, audit, "provider_error", "Provider failed without changing the client system.", options.Now()), err
		}
		proposal.Round = round
		if proposal.ReportID == "" {
			proposal.ReportID = report.ReportID
		}
		if proposal.GeneratedAt.IsZero() {
			proposal.GeneratedAt = options.Now().UTC()
		}
		if proposal.SchemaVersion == "" {
			proposal.SchemaVersion = SchemaVersion
		}
		if proposal.EvidenceRequests == nil {
			proposal.EvidenceRequests = []EvidenceRequest{}
		}
		// The loop owns the audit timeline; provider-supplied events are ignored.
		proposal.AuditTimeline = append([]AuditEvent{}, audit...)
		if len(proposal.Limitations) == 0 {
			proposal.Limitations = []string{"Online agent results remain provisional until a technician confirms them."}
		}
		if err := validateSize("provider response", proposal, options.MaxResponseBytes); err != nil {
			return loop.fail(report.ReportID, round, audit, "response_too_large", err.Error(), options.Now()), err
		}
		if err := ValidateResult(proposal, priorKeys); err != nil {
			audit = append(audit, AuditEvent{
				At:     options.Now().UTC(),
				Kind:   AuditLoopFailed,
				Round:  round,
				Detail: err.Error(),
			})
			return loop.fail(report.ReportID, round, audit, "invalid_provider_result", "Provider result failed policy validation.", options.Now()), err
		}
		audit = append(audit, AuditEvent{
			At:        options.Now().UTC(),
			Kind:      AuditProviderProposed,
			Round:     round,
			Reference: proposal.State,
		})
		last = proposal

		switch proposal.State {
		case StateCompleted:
			audit = append(audit, AuditEvent{At: options.Now().UTC(), Kind: AuditLoopCompleted, Round: round})
			proposal.AuditTimeline = append([]AuditEvent{}, audit...)
			return proposal, nil
		case StateBlocked:
			audit = append(audit, AuditEvent{
				At:        options.Now().UTC(),
				Kind:      AuditLoopBlocked,
				Round:     round,
				Reference: proposal.Block.Code,
			})
			proposal.AuditTimeline = append([]AuditEvent{}, audit...)
			return proposal, nil
		case StateFailed:
			audit = append(audit, AuditEvent{
				At:        options.Now().UTC(),
				Kind:      AuditLoopFailed,
				Round:     round,
				Reference: proposal.Failure.Code,
			})
			proposal.AuditTimeline = append([]AuditEvent{}, audit...)
			return proposal, nil
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
						Message: "Evidence gathering stopped after the maximum number of rounds.",
					},
					Limitations: []string{
						"The agent loop stopped after three rounds without a completed assessment.",
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
				payload, err := loop.Collector.Collect(ctx, request)
				if err != nil {
					return loop.fail(report.ReportID, round, audit, "evidence_collection_failed", "Read-only evidence collection failed.", options.Now()), err
				}
				if payload.RequestID == "" {
					payload.RequestID = request.ID
				}
				if payload.Operation == "" {
					payload.Operation = request.Operation
				}
				if payload.CollectedAt.IsZero() {
					payload.CollectedAt = options.Now().UTC()
				}
				priorEvidence = append(priorEvidence, payload)
				priorKeys = append(priorKeys, key)
				audit = append(audit, AuditEvent{
					At:        options.Now().UTC(),
					Kind:      AuditEvidenceCollected,
					Round:     round,
					Reference: payload.RequestID,
				})
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
	audit = append(audit, AuditEvent{
		At:        now.UTC(),
		Kind:      AuditLoopFailed,
		Round:     round,
		Reference: code,
		Detail:    message,
	})
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
