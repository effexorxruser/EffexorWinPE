//go:build !windows

package collector

func currentRuntimeWindowsRoot() string {
	return ""
}

func currentEnvironmentIsWinPE() bool {
	return false
}
