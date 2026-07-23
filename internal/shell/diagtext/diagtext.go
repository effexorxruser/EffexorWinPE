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

func findingKey(id, field string) string {
	id = canonicalizeFindingID(id)
	return "diag.finding." + id + "." + field
}

// canonicalizeFindingID maps dynamic IDs onto stable catalog suffixes.
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
