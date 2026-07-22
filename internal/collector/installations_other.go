//go:build !windows

package collector

import "github.com/effexorxruser/EffexorWinPE/internal/diagnostics"

func findWindowsInstallations() []diagnostics.Installation {
	return []diagnostics.Installation{}
}
