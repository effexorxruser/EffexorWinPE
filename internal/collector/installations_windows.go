//go:build windows

package collector

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func findWindowsInstallations() []diagnostics.Installation {
	var installations []diagnostics.Installation
	for letter := 'C'; letter <= 'Z'; letter++ {
		root := fmt.Sprintf("%c:\\", letter)
		hive := filepath.Join(root, "Windows", "System32", "config", "SYSTEM")
		if info, err := os.Stat(hive); err == nil && !info.IsDir() {
			installations = append(installations, diagnostics.Installation{
				Root:       root,
				SystemHive: hive,
			})
		}
	}
	return installations
}
