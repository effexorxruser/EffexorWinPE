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
	if report.Collector.Name != "effexorwinpe-collector" {
		t.Fatalf("collector name = %q, want effexorwinpe-collector", report.Collector.Name)
	}
	if len(report.Checks) == 0 {
		t.Fatal("checks are empty")
	}
	if report.Hardware.FirmwareMode == "" {
		t.Fatal("firmware mode is empty")
	}
	if report.Storage.Disks == nil || report.Storage.DriveHealth == nil || report.Storage.Partitions == nil {
		t.Fatal("storage arrays must be initialized")
	}
	if report.Storage.BitLockerInventory.Status == "" {
		t.Fatal("bitlocker inventory status must be set")
	}
	if report.Storage.BitLockerInventory.Status == diagnostics.BitLockerStatusUnavailable && report.Storage.BitLockerVolumes != nil {
		t.Fatal("unavailable BitLocker inventory must not serialize as an empty volume list")
	}
	if report.Storage.BitLockerInventory.Status == diagnostics.BitLockerStatusOK && report.Storage.BitLockerVolumes == nil {
		t.Fatal("successful BitLocker inventory must use an explicit volume list")
	}
	if report.Boot.BCDStores == nil {
		t.Fatal("BCD stores array must be initialized")
	}
}
