package agentloop

import (
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
	"github.com/effexorxruser/EffexorWinPE/internal/session"
)

const SchemaVersion = "0.1.0"

const (
	StateCompleted         = "completed"
	StateNeedsMoreEvidence = "needs_more_evidence"
	StateBlocked           = "blocked"
	StateFailed            = "failed"
)

const (
	PrivacyMachineInventory = "machine_inventory"
	PrivacyBootConfig       = "boot_config"
	PrivacyStorageHealth    = "storage_health"
	PrivacyEncryptionStatus = "encryption_status"
	PrivacyNetworkStatus    = "network_status"
)

const (
	AuditRoundStarted             = "round_started"
	AuditProviderProposed         = "provider_proposed"
	AuditEvidenceRequested        = "evidence_requested"
	AuditEvidenceCollected        = "evidence_collected"
	AuditEvidenceRedacted         = "evidence_redacted"
	AuditDuplicateRequestRejected = "duplicate_request_rejected"
	AuditLoopCompleted            = "loop_completed"
	AuditLoopBlocked              = "loop_blocked"
	AuditLoopFailed               = "loop_failed"
)

// Result is the loop-owned terminal or intermediate outcome, including audit.
type Result struct {
	SchemaVersion    string                `json:"schema_version"`
	ReportID         string                `json:"report_id"`
	GeneratedAt      time.Time             `json:"generated_at"`
	State            string                `json:"state"`
	Round            int                   `json:"round"`
	Assessment       *diagnosis.Assessment `json:"assessment,omitempty"`
	EvidenceRequests []EvidenceRequest     `json:"evidence_requests"`
	Block            *StatusDetail         `json:"block,omitempty"`
	Failure          *StatusDetail         `json:"failure,omitempty"`
	AuditTimeline    []AuditEvent          `json:"audit_timeline"`
	Limitations      []string              `json:"limitations"`
}

// ProviderProposal is the strict provider-controlled turn output. The loop does
// not invent missing required fields; malformed proposals fail closed.
type ProviderProposal struct {
	SchemaVersion    string                `json:"schema_version"`
	ReportID         string                `json:"report_id"`
	GeneratedAt      time.Time             `json:"generated_at"`
	State            string                `json:"state"`
	Round            int                   `json:"round"`
	Assessment       *diagnosis.Assessment `json:"assessment,omitempty"`
	EvidenceRequests []EvidenceRequest     `json:"evidence_requests"`
	Block            *StatusDetail         `json:"block,omitempty"`
	Failure          *StatusDetail         `json:"failure,omitempty"`
	Limitations      []string              `json:"limitations"`
	// RetrievedSources are HTTPS URLs actually returned by the provider API for
	// this turn. Assessment sources/source_refs may only cite these URLs.
	RetrievedSources []diagnosis.Source `json:"retrieved_sources"`
}

// EvidenceRequest asks the local client for one closed read-only observation.
type EvidenceRequest struct {
	ID                  string         `json:"id"`
	Operation           string         `json:"operation"`
	Arguments           map[string]any `json:"arguments"`
	Reason              string         `json:"reason"`
	ExpectedInformation string         `json:"expected_information"`
	PrivacyClass        string         `json:"privacy_class"`
	TimeoutSeconds      int            `json:"timeout_seconds"`
}

type StatusDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type AuditEvent struct {
	At        time.Time `json:"at"`
	Kind      string    `json:"kind"`
	Round     int       `json:"round,omitempty"`
	Reference string    `json:"reference,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

// SanitizedAgentContext is the only report/session view exposed to providers.
type SanitizedAgentContext struct {
	Report  diagnostics.Report `json:"report"`
	Session session.Session    `json:"session"`
}

// RoundInput is the provider-neutral context for one loop turn.
type RoundInput struct {
	Context          SanitizedAgentContext `json:"context"`
	Round            int                   `json:"round"`
	PriorEvidence    []EvidencePayload     `json:"prior_evidence"`
	PriorRequestKeys []string              `json:"prior_request_keys"`
}

// EvidencePayload is a deterministic local observation returned for a request.
// EvidenceRefs are loop-generated from the validated facts; collector-supplied
// refs are ignored.
type EvidencePayload struct {
	RequestID    string         `json:"request_id"`
	Operation    string         `json:"operation"`
	PrivacyClass string         `json:"privacy_class"`
	CollectedAt  time.Time      `json:"collected_at"`
	Facts        map[string]any `json:"facts"`
	EvidenceRefs []string       `json:"evidence_refs"`
}

func (proposal ProviderProposal) toResult(audit []AuditEvent) Result {
	requests := proposal.EvidenceRequests
	if requests == nil {
		requests = []EvidenceRequest{}
	}
	return Result{
		SchemaVersion:    proposal.SchemaVersion,
		ReportID:         proposal.ReportID,
		GeneratedAt:      proposal.GeneratedAt,
		State:            proposal.State,
		Round:            proposal.Round,
		Assessment:       proposal.Assessment,
		EvidenceRequests: requests,
		Block:            proposal.Block,
		Failure:          proposal.Failure,
		AuditTimeline:    append([]AuditEvent{}, audit...),
		Limitations:      append([]string{}, proposal.Limitations...),
	}
}
