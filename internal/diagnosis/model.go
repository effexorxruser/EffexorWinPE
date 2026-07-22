package diagnosis

import "time"

const SchemaVersion = "0.1.0"

const (
	ModeOfflinePreflight = "offline_preflight"

	SeverityInfo     = "info"
	SeverityWarning  = "warning"
	SeverityCritical = "critical"
	SeverityUnknown  = "unknown"

	ConfidenceLow    = "low"
	ConfidenceMedium = "medium"
	ConfidenceHigh   = "high"

	RiskReadOnly       = "read_only"
	RiskChangesSystem  = "changes_system"
	RiskChangesBoot    = "changes_boot"
	RiskChangesStorage = "changes_storage"

	AnswerYesNo    = "yes_no"
	AnswerFreeText = "free_text"
)

// Assessment is the local, deterministic preflight result. It is deliberately
// evidence-first and contains no executable command strings.
type Assessment struct {
	SchemaVersion string     `json:"schema_version"`
	ReportID      string     `json:"report_id"`
	GeneratedAt   time.Time  `json:"generated_at"`
	Mode          string     `json:"mode"`
	Summary       Summary    `json:"summary"`
	Findings      []Finding  `json:"findings"`
	Questions     []Question `json:"questions"`
	NextSteps     []NextStep `json:"next_steps"`
	Limitations   []string   `json:"limitations"`
}

type Summary struct {
	Headline        string `json:"headline"`
	HighestSeverity string `json:"highest_severity"`
	FindingCount    int    `json:"finding_count"`
}

type Finding struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Severity     string   `json:"severity"`
	Confidence   string   `json:"confidence"`
	Rationale    string   `json:"rationale"`
	EvidenceRefs []string `json:"evidence_refs"`
}

type Question struct {
	ID         string `json:"id"`
	Prompt     string `json:"prompt"`
	Reason     string `json:"reason"`
	AnswerType string `json:"answer_type"`
}

// NextStep names a bounded operation understood by the client. It never carries
// a shell, PowerShell, or diskpart command supplied by a model.
type NextStep struct {
	ID                   string `json:"id"`
	Title                string `json:"title"`
	Operation            string `json:"operation"`
	Risk                 string `json:"risk"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	Rationale            string `json:"rationale"`
}
