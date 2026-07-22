package collector

import (
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestCollectProducesVersionedReport(t *testing.T) {
	report, err := Collect("test")
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if report.SchemaVersion != diagnostics.SchemaVersion {
		t.Fatalf("schema version = %q, want %q", report.SchemaVersion, diagnostics.SchemaVersion)
	}
	if report.ReportID == "" {
		t.Fatal("report id is empty")
	}
	if report.Collector.Version != "test" {
		t.Fatalf("collector version = %q, want test", report.Collector.Version)
	}
	if len(report.Checks) == 0 {
		t.Fatal("checks are empty")
	}
	if report.Hardware.FirmwareMode == "" {
		t.Fatal("firmware mode is empty")
	}
	if report.Storage.Disks == nil || report.Storage.DriveHealth == nil || report.Storage.Partitions == nil || report.Storage.BitLockerVolumes == nil {
		t.Fatal("storage arrays must be initialized")
	}
	if report.Boot.BCDStores == nil {
		t.Fatal("BCD stores array must be initialized")
	}
}
