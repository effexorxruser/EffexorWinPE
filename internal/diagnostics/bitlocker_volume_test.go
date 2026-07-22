package diagnostics

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBitLockerVolumeNullFieldsRoundTrip(t *testing.T) {
	t.Parallel()
	status := "FullyEncrypted"
	tests := []struct {
		name string
		in   BitLockerVolume
		want string
	}{
		{
			name: "all fields present",
			in: BitLockerVolume{
				MountPoint:       "C:",
				VolumeStatus:     &status,
				ProtectionStatus: strPtr("On"),
				LockStatus:       strPtr("Locked"),
				EncryptionMethod: strPtr("XTS_AES_256"),
			},
			want: `{"mount_point":"C:","volume_status":"FullyEncrypted","protection_status":"On","lock_status":"Locked","encryption_method":"XTS_AES_256"}`,
		},
		{
			name: "unavailable fields stay null",
			in:   BitLockerVolume{MountPoint: "D:"},
			want: `{"mount_point":"D:","volume_status":null,"protection_status":null,"lock_status":null,"encryption_method":null}`,
		},
		{
			name: "partial null fields",
			in: BitLockerVolume{
				MountPoint:   "E:",
				LockStatus:   strPtr("Unlocked"),
				VolumeStatus: nil,
			},
			want: `{"mount_point":"E:","volume_status":null,"protection_status":null,"lock_status":"Unlocked","encryption_method":null}`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(test.in)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			if string(raw) != test.want {
				t.Fatalf("Marshal() = %s, want %s", raw, test.want)
			}
			var decoded BitLockerVolume
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatalf("Unmarshal() error = %v", err)
			}
			roundTrip, err := json.Marshal(decoded)
			if err != nil {
				t.Fatalf("re-marshal error = %v", err)
			}
			if string(roundTrip) != test.want {
				t.Fatalf("round-trip = %s, want %s", roundTrip, test.want)
			}
			if decoded.VolumeStatus != nil && test.in.VolumeStatus == nil {
				t.Fatal("null volume_status became non-nil")
			}
			if decoded.ProtectionStatus != nil && test.in.ProtectionStatus == nil {
				t.Fatal("null protection_status became non-nil")
			}
		})
	}
}

func TestDecodeReportV13RejectsUnavailableWithArray(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version":"1.3.0","report_id":"report000000000001","collected_at":"2026-07-22T12:00:00Z",
		"collector":{"name":"effexorwinpe-collector","version":"test"},
		"environment":{"runtime_os":"windows","runtime_arch":"amd64"},
		"hardware":{"firmware_mode":"uefi","system":{},"processor":{"cores":0,"logical_processors":0},"memory":{"total_physical_bytes":0},"network_adapters":[]},
		"storage":{"disks":[],"drive_health":[],"partitions":[],"bitlocker_volumes":[],"bitlocker_inventory":{"status":"unavailable"}},
		"boot":{"firmware_mode":"uefi","bcd_stores":[]},"windows_installations":[],"checks":[],
		"privacy":{"contains_personal_data":false,"excluded_by_default":[]}
	}`)
	if _, err := DecodeReportJSON(raw); err == nil {
		t.Fatal("DecodeReportJSON() error = nil, want unavailable+array rejection")
	}
}

func TestDecodeReportV13RejectsOKWithNullVolumes(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"schema_version":"1.3.0","report_id":"report000000000001","collected_at":"2026-07-22T12:00:00Z",
		"collector":{"name":"effexorwinpe-collector","version":"test"},
		"environment":{"runtime_os":"windows","runtime_arch":"amd64"},
		"hardware":{"firmware_mode":"uefi","system":{},"processor":{"cores":0,"logical_processors":0},"memory":{"total_physical_bytes":0},"network_adapters":[]},
		"storage":{"disks":[],"drive_health":[],"partitions":[],"bitlocker_volumes":null,"bitlocker_inventory":{"status":"ok"}},
		"boot":{"firmware_mode":"uefi","bcd_stores":[]},"windows_installations":[],"checks":[],
		"privacy":{"contains_personal_data":false,"excluded_by_default":[]}
	}`)
	if _, err := DecodeReportJSON(raw); err == nil {
		t.Fatal("DecodeReportJSON() error = nil, want ok+null rejection")
	}
}

func TestMigrateLegacyPhysicalSmokeReportV12(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile(testdataPath(t, "physical-smoke-legacy-1.2.0.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	report, err := DecodeReportJSON(raw)
	if err != nil {
		t.Fatalf("DecodeReportJSON() error = %v", err)
	}
	if report.SchemaVersion != SchemaVersion {
		t.Fatalf("schema_version = %q, want %q", report.SchemaVersion, SchemaVersion)
	}
	if report.Storage.BitLockerInventory.Status != BitLockerStatusUnavailable {
		t.Fatalf("migrated BitLocker status = %q, want unavailable", report.Storage.BitLockerInventory.Status)
	}
	if report.Storage.BitLockerVolumes != nil {
		t.Fatalf("migrated volumes = %#v, want null", report.Storage.BitLockerVolumes)
	}
	if len(report.Hardware.NetworkAdapters) != 1 || report.Hardware.NetworkAdapters[0].Status != NetStatusMediaDisconnected {
		t.Fatalf("network status not normalized: %+v", report.Hardware.NetworkAdapters)
	}
	if report.Hardware.NetworkAdapters[0].StatusCode == nil || *report.Hardware.NetworkAdapters[0].StatusCode != 7 {
		t.Fatalf("status_code not preserved: %+v", report.Hardware.NetworkAdapters[0])
	}
	if len(report.Installations) == 0 || report.Installations[0].Version == nil {
		t.Fatal("expected migrated installation version")
	}
	version := report.Installations[0].Version
	if version.RawProductName != "Windows 10 Pro" || version.ProductName != "Windows 11 Pro" {
		t.Fatalf("product name migration = %+v", version)
	}
}

func TestMigrateLegacyEmptyBitLockerIsNotOK(t *testing.T) {
	t.Parallel()
	inventory, volumes := migrateBitLockerV12(nil)
	if inventory.Status != BitLockerStatusUnavailable || volumes != nil {
		t.Fatalf("migrateBitLockerV12(nil) = %+v, %#v", inventory, volumes)
	}
	inventory, volumes = migrateBitLockerV12([]bitLockerVolumeV12{})
	if inventory.Status != BitLockerStatusUnavailable || volumes != nil {
		t.Fatalf("migrateBitLockerV12([]) = %+v, %#v", inventory, volumes)
	}
}

func strPtr(value string) *string { return &value }

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "testdata", name)
}
