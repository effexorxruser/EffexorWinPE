package agentloop

import (
	"fmt"
	"regexp"
	"strings"
)

var forbiddenCommandPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bpowershell\b`),
	regexp.MustCompile(`(?i)\bpwsh\b`),
	regexp.MustCompile(`(?i)\bcmd\.exe\b`),
	regexp.MustCompile(`(?i)\bcommand\.com\b`),
	regexp.MustCompile(`(?i)\bdiskpart\b`),
	regexp.MustCompile(`(?i)\breg\.exe\b`),
	regexp.MustCompile(`(?i)\bbcdedit\b`),
	regexp.MustCompile(`(?i)\bformat\.com\b`),
	regexp.MustCompile(`(?i)\bdel\s+/[fqs]`),
	regexp.MustCompile(`(?i)\brm\s+-rf\b`),
	regexp.MustCompile(`(?i)\bInvoke-Expression\b`),
	regexp.MustCompile(`(?i)\bIEX\b`),
	regexp.MustCompile("(?i)`[^`]+`"),
	regexp.MustCompile(`(?i)\$[A-Za-z_][A-Za-z0-9_]*\s*=`),
	regexp.MustCompile(`(?i)\bcurl\s+https?://`),
	regexp.MustCompile(`(?i)\bwget\s+https?://`),
}

// RejectCommandText blocks shell, PowerShell, and free-form command text from
// model-authored fields. Operation identifiers remain the only executable surface.
func RejectCommandText(field, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	for _, pattern := range forbiddenCommandPatterns {
		if pattern.MatchString(trimmed) {
			return fmt.Errorf("%s contains forbidden command text", field)
		}
	}
	if strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "|") && strings.Contains(trimmed, ".exe") {
		return fmt.Errorf("%s contains forbidden command text", field)
	}
	return nil
}
