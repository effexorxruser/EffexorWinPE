package agentloop

import (
	"strings"
	"unicode"
)

// NormalizeWindowsPath canonicalizes slash direction and trailing separators
// without resolving parent segments (those remain forbidden separately).
func NormalizeWindowsPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	normalized := strings.ReplaceAll(trimmed, "/", `\`)
	// Keep drive-root form "C:\" but strip trailing separators otherwise.
	for {
		if normalized == `\` {
			return normalized
		}
		if len(normalized) == 3 && unicode.IsLetter(rune(normalized[0])) && normalized[1] == ':' && normalized[2] == '\\' {
			return strings.ToUpper(normalized[:1]) + `:\`
		}
		if !strings.HasSuffix(normalized, `\`) {
			break
		}
		if len(normalized) <= 3 {
			break
		}
		normalized = strings.TrimSuffix(normalized, `\`)
	}
	if len(normalized) >= 2 && unicode.IsLetter(rune(normalized[0])) && normalized[1] == ':' {
		return strings.ToUpper(normalized[:1]) + normalized[1:]
	}
	return normalized
}

// WindowsPathsEqual compares paths case-insensitively after normalization.
func WindowsPathsEqual(left, right string) bool {
	return strings.EqualFold(NormalizeWindowsPath(left), NormalizeWindowsPath(right))
}

func normalizeArgumentValue(name string, value any) any {
	str, ok := value.(string)
	if !ok {
		return value
	}
	switch name {
	case "root", "store_path", "mount_point":
		return NormalizeWindowsPath(str)
	default:
		return value
	}
}
