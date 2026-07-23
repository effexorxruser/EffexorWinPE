// Package agentloop implements a provider-neutral multi-step diagnostic loop.
//
// The loop never executes shell, PowerShell, or free-form command text from a
// model. Evidence gathering is limited to a closed read-only operation allowlist.
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
	AuditDuplicateRequestRejected = "duplicate_request_rejected"
	AuditLoopCompleted            = "loop_completed"
	AuditLoopBlocked              = "loop_blocked"
	AuditLoopFailed               = "loop_failed"
)

// Result is the versioned online agent-loop outcome for one proposal or final turn.
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

// EvidenceRequest asks the local client for one closed read-only observation.
// Arguments are validated against the operation allowlist before execution.
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

// RoundInput is the provider-neutral context for one loop turn.
type RoundInput struct {
	Report           diagnostics.Report
	Session          session.Session
	Round            int
	PriorEvidence    []EvidencePayload
	PriorRequestKeys []string
}

// EvidencePayload is a deterministic local observation returned for a request.
type EvidencePayload struct {
	RequestID    string         `json:"request_id"`
	Operation    string         `json:"operation"`
	CollectedAt  time.Time      `json:"collected_at"`
	Facts        map[string]any `json:"facts"`
	EvidenceRefs []string       `json:"evidence_refs"`
}
