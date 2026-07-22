package diagnostics

import (
	"strconv"
	"strings"
)

// Windows 11 client releases begin at build 22000.
const windows11ClientBuildFloor uint64 = 22000

// NormalizeWindowsVersion returns a copy with ProductName normalized for display
// while preserving RawProductName as the unmodified registry ProductName.
// Client builds >= 22000 that still report a legacy "Windows 10 *" ProductName
// are rewritten to "Windows 11 *". Windows Server products are never rewritten
// by client build rules.
func NormalizeWindowsVersion(version WindowsVersion) WindowsVersion {
	raw := strings.TrimSpace(version.RawProductName)
	if raw == "" {
		raw = strings.TrimSpace(version.ProductName)
	}
	version.RawProductName = raw
	version.ProductName = normalizeClientProductName(raw, version.InstallationType, version.Build)
	return version
}

func normalizeClientProductName(rawProductName, installationType, build string) string {
	rawProductName = strings.TrimSpace(rawProductName)
	if rawProductName == "" {
		return ""
	}
	if isWindowsServerProduct(rawProductName, installationType) {
		return rawProductName
	}
	buildNumber, ok := ParseWindowsBuildNumber(build)
	if !ok || buildNumber < windows11ClientBuildFloor {
		return rawProductName
	}
	return rewriteWindows10ProductName(rawProductName)
}

func isWindowsServerProduct(productName, installationType string) bool {
	if strings.EqualFold(strings.TrimSpace(installationType), "Server") {
		return true
	}
	lower := strings.ToLower(productName)
	return strings.Contains(lower, "server")
}

func rewriteWindows10ProductName(productName string) string {
	const legacy = "Windows 10"
	const modern = "Windows 11"
	if strings.HasPrefix(strings.ToLower(productName), strings.ToLower(legacy)) {
		return modern + productName[len(legacy):]
	}
	return productName
}

// ParseWindowsBuildNumber extracts the major build number from values such as
// "22621", "22621.1848", or "10.0.22621.1848". Incomplete or non-numeric builds
// return ok=false.
func ParseWindowsBuildNumber(build string) (uint64, bool) {
	build = strings.TrimSpace(build)
	if build == "" {
		return 0, false
	}
	parts := strings.Split(build, ".")
	candidates := parts
	if len(parts) >= 3 {
		// Prefer the Windows build component in dotted OS versions: 10.0.22621.1848
		candidates = []string{parts[2], parts[0]}
	}
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		parsed, err := strconv.ParseUint(candidate, 10, 64)
		if err != nil {
			continue
		}
		return parsed, true
	}
	return 0, false
}
