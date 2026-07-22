package collector

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func Collect(version string) (diagnostics.Report, error) {
	id, err := newReportID()
	if err != nil {
		return diagnostics.Report{}, err
	}

	hostname, _ := os.Hostname()
	installations := findWindowsInstallations()
	hardware, storage, boot, platformChecks := collectPlatform()

	checks := []diagnostics.Check{
		{
			ID:      "collector.runtime",
			Status:  "ok",
			Summary: fmt.Sprintf("Collector is running on %s/%s", runtime.GOOS, runtime.GOARCH),
		},
	}
	checks = append(checks, platformChecks...)
	if len(installations) == 0 {
		checks = append(checks, diagnostics.Check{
			ID:      "windows.installations",
			Status:  "warning",
			Summary: "No offline Windows installation was detected",
		})
	} else {
		checks = append(checks, diagnostics.Check{
			ID:      "windows.installations",
			Status:  "ok",
			Summary: fmt.Sprintf("Detected %d Windows installation(s)", len(installations)),
		})
	}

	return diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      id,
		CollectedAt:   time.Now().UTC(),
		Collector: diagnostics.Collector{
			Name:    "anp-collector",
			Version: version,
		},
		Environment: diagnostics.Environment{
			RuntimeOS:   runtime.GOOS,
			RuntimeArch: runtime.GOARCH,
			Hostname:    hostname,
		},
		Hardware:      hardware,
		Storage:       storage,
		Boot:          boot,
		Installations: installations,
		Checks:        checks,
		Privacy: diagnostics.Privacy{
			ContainsPersonalData: hostname != "",
			ExcludedByDefault: []string{
				"hostname",
				"usernames",
				"file_contents",
				"browser_data",
				"wifi_profiles",
				"network_addresses",
			},
		},
	}, nil
}

func newReportID() (string, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("create report id: %w", err)
	}
	return hex.EncodeToString(value[:]), nil
}
