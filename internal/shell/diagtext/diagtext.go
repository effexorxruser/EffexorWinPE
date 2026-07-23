package diagtext

import (
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/shell/i18n"
)

// T returns a localized diagnosis string for a stable ID-derived key.
// If the catalog has no translation, fallback (usually English raw text) is returned.
func T(b *i18n.Bundle, key, fallback string) string {
	if b == nil {
		return fallback
	}
	if got := b.T(key); got != key {
		return got
	}
	return fallback
}

// FindingTitle localizes a finding title by stable ID.
func FindingTitle(b *i18n.Bundle, id, fallback string) string {
	return T(b, findingKey(id, "title"), fallback)
}

// FindingRationale localizes a finding rationale by stable ID.
func FindingRationale(b *i18n.Bundle, id, fallback string) string {
	return T(b, findingKey(id, "rationale"), fallback)
}

// StepTitle localizes a next-step title by stable ID.
func StepTitle(b *i18n.Bundle, id, fallback string) string {
	return T(b, "diag.step."+id+".title", fallback)
}

// StepRationale localizes a next-step rationale by stable ID.
func StepRationale(b *i18n.Bundle, id, fallback string) string {
	return T(b, "diag.step."+id+".rationale", fallback)
}

// QuestionPrompt localizes a follow-up question by stable ID.
func QuestionPrompt(b *i18n.Bundle, id, fallback string) string {
	return T(b, "diag.question."+id+".prompt", fallback)
}

// Headline localizes an assessment headline by highest severity.
func Headline(b *i18n.Bundle, severity, fallback string) string {
	key := "diag.headline.info"
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		key = "diag.headline.critical"
	case "warning":
		key = "diag.headline.warning"
	case "unknown":
		key = "diag.headline.unknown"
	}
	return T(b, key, fallback)
}

// Severity localizes a severity enum.
func Severity(b *i18n.Bundle, value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info":
		return b.T("severity.info")
	case "warning":
		return b.T("severity.warning")
	case "critical":
		return b.T("severity.critical")
	case "unknown":
		return b.T("severity.unknown")
	default:
		return value
	}
}

// Confidence localizes a confidence enum.
func Confidence(b *i18n.Bundle, value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "low":
		return b.T("confidence.low")
	case "medium":
		return b.T("confidence.medium")
	case "high":
		return b.T("confidence.high")
	default:
		return value
	}
}

// Risk localizes a risk enum.
func Risk(b *i18n.Bundle, value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "read_only":
		return b.T("risk.read_only")
	case "changes_system":
		return b.T("risk.changes_system")
	case "changes_boot":
		return b.T("risk.changes_boot")
	case "changes_storage":
		return b.T("risk.changes_storage")
	default:
		return value
	}
}

// CheckSummary localizes a collector check summary by stable check ID.
// Raw English remains available as technical fallback.
func CheckSummary(b *i18n.Bundle, id, fallback string) (localized string, technical string) {
	key := "check." + canonicalizeCheckID(id) + ".summary"
	if got := b.T(key); got != key {
		return got, fallback
	}
	return fallback, ""
}

// Limitation localizes a known offline-preflight limitation string.
func Limitation(b *i18n.Bundle, raw string) string {
	switch {
	case strings.Contains(raw, "Offline preflight uses conservative deterministic rules"):
		return b.T("diag.limitation.offline_conservative")
	case strings.Contains(raw, "Missing providers or counters reduce confidence"):
		return b.T("diag.limitation.missing_providers")
	case strings.Contains(raw, "read-only next steps only"):
		return b.T("diag.limitation.read_only_steps")
	case strings.Contains(raw, "does not contact the model gateway"):
		return b.T("diag.limitation.no_gateway")
	case strings.Contains(raw, "SMART counters may be null"):
		return b.T("diag.limitation.smart_null")
	default:
		// Unknown online/model text: keep raw English.
		return raw
	}
}

func findingKey(id, field string) string {
	id = canonicalizeFindingID(id)
	return "diag.finding." + id + "." + field
}

func canonicalizeFindingID(id string) string {
	id = strings.TrimSpace(id)
	switch {
	case strings.HasPrefix(id, "storage.disk.") && strings.HasSuffix(id, ".health"):
		return "storage.disk.health"
	case strings.HasPrefix(id, "storage.drive.") && strings.HasSuffix(id, ".health"):
		return "storage.drive.health"
	case strings.HasPrefix(id, "storage.drive.") && strings.HasSuffix(id, ".temperature"):
		return "storage.drive.temperature"
	case strings.HasPrefix(id, "storage.drive.") && strings.HasSuffix(id, ".wear"):
		return "storage.drive.wear"
	case strings.HasPrefix(id, "storage.drive.") && strings.HasSuffix(id, ".errors"):
		return "storage.drive.errors"
	default:
		return id
	}
}

func canonicalizeCheckID(id string) string {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "platform.inventory.source.") {
		return "platform.inventory.source"
	}
	return id
}
