package collector

import (
	"encoding/json"
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestNormalizeBitLockerInventoryEmptySuccess(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		BitLockerVolumes:   []diagnostics.BitLockerVolume{},
		BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusOK},
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerInventory.Status != diagnostics.BitLockerStatusOK {
		t.Fatalf("status = %q, want ok", storage.BitLockerInventory.Status)
	}
	if storage.BitLockerVolumes == nil || len(storage.BitLockerVolumes) != 0 {
		t.Fatalf("volumes = %#v, want empty non-nil slice", storage.BitLockerVolumes)
	}
}

func TestNormalizeBitLockerInventoryProviderMissing(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		BitLockerVolumes: []diagnostics.BitLockerVolume{},
		BitLockerInventory: diagnostics.BitLockerInventory{
			Status: diagnostics.BitLockerStatusUnavailable,
			Error:  "provider missing",
		},
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerVolumes != nil {
		t.Fatalf("unavailable inventory must use null volumes, got %#v", storage.BitLockerVolumes)
	}
}

func TestNormalizeBitLockerInventoryNullFieldsPartial(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		BitLockerVolumes: []diagnostics.BitLockerVolume{{
			MountPoint: "D:",
		}},
		BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusPartial, Error: "incomplete"},
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerInventory.Status != diagnostics.BitLockerStatusPartial {
		t.Fatalf("status = %q, want partial", storage.BitLockerInventory.Status)
	}
	if len(storage.BitLockerVolumes) != 1 {
		t.Fatalf("volumes len = %d, want 1", len(storage.BitLockerVolumes))
	}
}

func TestNormalizeBitLockerInventoryNormalVolume(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		BitLockerVolumes: []diagnostics.BitLockerVolume{{
			MountPoint:       "C:",
			VolumeStatus:     "FullyEncrypted",
			ProtectionStatus: "On",
			LockStatus:       "Locked",
			EncryptionMethod: "XTS_AES_256",
		}},
		BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusOK},
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerInventory.Status != diagnostics.BitLockerStatusOK || len(storage.BitLockerVolumes) != 1 {
		t.Fatalf("unexpected normalized storage: %+v", storage)
	}
}

func TestDriveHealthMetricSerialization(t *testing.T) {
	t.Parallel()
	zero := uint64(0)
	nonzero := uint64(42)
	tests := []struct {
		name string
		in   diagnostics.DriveHealth
		want string
	}{
		{
			name: "nonzero value",
			in:   diagnostics.DriveHealth{DeviceID: "1", TemperatureC: &nonzero, WearPercent: &nonzero, PowerOnHours: &nonzero, ReadErrorsTotal: &nonzero, WriteErrorsTotal: &nonzero},
			want: `{"device_id":"1","temperature_celsius":42,"wear_percent":42,"power_on_hours":42,"read_errors_total":42,"write_errors_total":42}`,
		},
		{
			name: "real zero value",
			in:   diagnostics.DriveHealth{DeviceID: "2", TemperatureC: &zero, WearPercent: &zero, PowerOnHours: &zero, ReadErrorsTotal: &zero, WriteErrorsTotal: &zero},
			want: `{"device_id":"2","temperature_celsius":0,"wear_percent":0,"power_on_hours":0,"read_errors_total":0,"write_errors_total":0}`,
		},
		{
			name: "unavailable metrics are null",
			in:   diagnostics.DriveHealth{DeviceID: "3"},
			want: `{"device_id":"3","temperature_celsius":null,"wear_percent":null,"power_on_hours":null,"read_errors_total":null,"write_errors_total":null}`,
		},
		{
			name: "partial metrics",
			in:   diagnostics.DriveHealth{DeviceID: "4", WearPercent: &zero, ReadErrorsTotal: &nonzero},
			want: `{"device_id":"4","temperature_celsius":null,"wear_percent":0,"power_on_hours":null,"read_errors_total":42,"write_errors_total":null}`,
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
			var decoded diagnostics.DriveHealth
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
		})
	}
}

func TestBitLockerUnavailableSerializesNullVolumes(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		Disks:            []diagnostics.Disk{},
		DriveHealth:      []diagnostics.DriveHealth{},
		Partitions:       []diagnostics.Partition{},
		BitLockerVolumes: nil,
		BitLockerInventory: diagnostics.BitLockerInventory{
			Status: diagnostics.BitLockerStatusUnavailable,
			Error:  "Index operation failed; the array index evaluated to null.",
		},
	}
	raw, err := json.Marshal(storage)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]json.RawMessage
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("Unmarshal map error = %v", err)
	}
	if string(decoded["bitlocker_volumes"]) != "null" {
		t.Fatalf("bitlocker_volumes = %s, want null", decoded["bitlocker_volumes"])
	}
}
