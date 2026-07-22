//go:build windows

package collector

import (
	"os"
	"os/exec"
	"strings"
)

func currentRuntimeWindowsRoot() string {
	for _, key := range []string{"SystemRoot", "SYSTEMROOT", "windir", "WINDIR"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func currentEnvironmentIsWinPE() bool {
	if miniNTKeyExists() {
		return true
	}
	systemRoot := currentRuntimeWindowsRoot()
	if systemRoot == "" {
		return false
	}
	return looksLikeWinPERoot(systemRoot, pathExists)
}

func miniNTKeyExists() bool {
	_, err := exec.Command("reg.exe", "query", `HKLM\SYSTEM\CurrentControlSet\Control\MiniNT`).CombinedOutput()
	return err == nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
