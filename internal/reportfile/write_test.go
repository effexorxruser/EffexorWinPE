package reportfile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestWriteCreatesValidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "report.json")
	want := diagnostics.Report{
		SchemaVersion: diagnostics.SchemaVersion,
		ReportID:      "test-id",
		Hardware:      diagnostics.Hardware{FirmwareMode: "unknown", NetworkAdapters: []diagnostics.NetworkAdapter{}},
		Storage: diagnostics.Storage{
			Disks:            []diagnostics.Disk{},
			DriveHealth:      []diagnostics.DriveHealth{},
			Partitions:       []diagnostics.Partition{},
			BitLockerVolumes: []diagnostics.BitLockerVolume{},
			BitLockerInventory: diagnostics.BitLockerInventory{
				Status: diagnostics.BitLockerStatusOK,
			},
		},
		Boot:          diagnostics.Boot{FirmwareMode: "unknown", BCDStores: []diagnostics.BCDStore{}},
		Installations: []diagnostics.Installation{},
		Checks:        []diagnostics.Check{},
	}
	if err := Write(path, want, true); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var got diagnostics.Report
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.ReportID != want.ReportID {
		t.Fatalf("report id = %q, want %q", got.ReportID, want.ReportID)
	}
}
