package collector

import (
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

// filterOfflineWindowsInstallations removes the currently running Windows/WinPE
// root from the offline installation list. Runtime exclusion is based on the
// resolved SystemRoot install path, not a hard-coded X: drive letter.
// When runtimeIsWinPE is true and exists is non-nil, roots with MiniNT/winpeshl/startnet
// markers are also omitted so a mounted rescue image is not treated as installed Windows.
func filterOfflineWindowsInstallations(installations []diagnostics.Installation, runtimeInstallRoot string, runtimeIsWinPE bool, exists func(string) bool) []diagnostics.Installation {
	if len(installations) == 0 {
		return []diagnostics.Installation{}
	}
	runtimeInstallRoot = normalizeWindowsInstallRoot(runtimeInstallRoot)
	filtered := make([]diagnostics.Installation, 0, len(installations))
	for _, installation := range installations {
		root := normalizeWindowsInstallRoot(installation.Root)
		if shouldExcludeWindowsInstallation(root, runtimeInstallRoot, runtimeIsWinPE, exists) {
			continue
		}
		installation.Root = ensureTrailingSlash(root)
		if installation.Version != nil {
			normalized := diagnostics.NormalizeWindowsVersion(*installation.Version)
			installation.Version = &normalized
		}
		filtered = append(filtered, installation)
	}
	return filtered
}

func shouldExcludeWindowsInstallation(root, runtimeInstallRoot string, runtimeIsWinPE bool, exists func(string) bool) bool {
	if runtimeInstallRoot != "" && sameWindowsPath(root, runtimeInstallRoot) {
		return true
	}
	if runtimeIsWinPE && exists != nil && looksLikeWinPERoot(root, exists) {
		return true
	}
	return false
}

// normalizeWindowsInstallRoot converts a SystemRoot or installation path into
// the install root (for example X:\Windows -> X:). Path handling is intentionally
// Windows-oriented so unit tests behave the same on Linux build hosts.
func normalizeWindowsInstallRoot(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	path = strings.ReplaceAll(path, "/", `\`)
	for strings.Contains(path, `\\`) {
		path = strings.ReplaceAll(path, `\\`, `\`)
	}
	path = strings.TrimRight(path, `\`)
	if path == "" {
		return ""
	}
	if index := strings.LastIndex(path, `\`); index >= 0 {
		base := path[index+1:]
		if strings.EqualFold(base, "Windows") {
			path = path[:index]
		}
	} else if strings.EqualFold(path, "Windows") {
		return ""
	}
	return strings.TrimRight(path, `\`)
}

func ensureTrailingSlash(root string) string {
	root = normalizeWindowsInstallRoot(root)
	if root == "" {
		return ""
	}
	return root + `\`
}

func sameWindowsPath(left, right string) bool {
	return strings.EqualFold(normalizeWindowsInstallRoot(left), normalizeWindowsInstallRoot(right))
}

// looksLikeWinPERoot reports MiniNT/WinPE markers under a Windows root without
// treating an ordinary Windows directory as WinPE by itself.
func looksLikeWinPERoot(root string, exists func(path string) bool) bool {
	root = ensureTrailingSlash(root)
	if root == `\` || root == "" {
		return false
	}
	markers := []string{
		root + `Windows\System32\MiniNT`,
		root + `Windows\System32\winpeshl.exe`,
		root + `Windows\System32\startnet.cmd`,
	}
	for _, marker := range markers {
		if exists(marker) {
			return true
		}
	}
	return false
}
