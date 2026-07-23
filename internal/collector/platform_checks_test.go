package collector

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func TestBuildPlatformChecksDeduplicatesUnavailableBitLocker(t *testing.T) {
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
	checks := buildPlatformChecks(storage, 1, 1, []string{
		"BitLocker inventory is unavailable: Index operation failed; the array index evaluated to null.",
		"Firmware mode is unavailable: missing",
	})
	bitLockerChecks := 0
	genericBitLocker := 0
	for _, check := range checks {
		if check.ID == "storage.bitlocker_inventory" {
			bitLockerChecks++
		}
		if strings.HasPrefix(check.ID, "platform.inventory.source.") && isBitLockerSourceError(check.Summary) {
			genericBitLocker++
		}
	}
	if bitLockerChecks != 1 {
		t.Fatalf("storage.bitlocker_inventory checks = %d, want 1", bitLockerChecks)
	}
	if genericBitLocker != 0 {
		t.Fatalf("generic BitLocker source checks = %d, want 0", genericBitLocker)
	}
	if !strings.Contains(checks[0].Summary, "volume count unknown") {
		t.Fatalf("summary = %q, want volume count unknown", checks[0].Summary)
	}
	if strings.Contains(checks[0].Summary, "0 volume(s)") {
		t.Fatalf("summary must not claim 0 volumes when BitLocker is unavailable: %q", checks[0].Summary)
	}
}

func TestBuildPlatformChecksPartialBitLockerCreatesOneUnknownCheck(t *testing.T) {
	t.Parallel()
	storage := diagnostics.Storage{
		BitLockerVolumes: []diagnostics.BitLockerVolume{{MountPoint: "C:"}},
		BitLockerInventory: diagnostics.BitLockerInventory{
			Status: diagnostics.BitLockerStatusPartial,
			Error:  "One or more BitLocker volume fields were incomplete",
		},
	}
	checks := buildPlatformChecks(storage, 0, 1, nil)
	unknownBitLocker := 0
	for _, check := range checks {
		if check.ID == "storage.bitlocker_inventory" {
			unknownBitLocker++
			if check.Status != "unknown" {
				t.Fatalf("status = %q, want unknown", check.Status)
			}
			if !strings.Contains(check.Summary, "partial") {
				t.Fatalf("summary = %q, want partial", check.Summary)
			}
		}
	}
	if unknownBitLocker != 1 {
		t.Fatalf("storage.bitlocker_inventory unknown checks = %d, want 1", unknownBitLocker)
	}
}

func TestFormatBitLockerInventorySummary(t *testing.T) {
	t.Parallel()
	ok := diagnostics.Storage{
		BitLockerVolumes:   []diagnostics.BitLockerVolume{},
		BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusOK},
	}
	if got := formatBitLockerInventorySummary(ok); got != "BitLocker status ok (0 volume(s))" {
		t.Fatalf("ok summary = %q", got)
	}
	unavailable := diagnostics.Storage{
		BitLockerInventory: diagnostics.BitLockerInventory{Status: diagnostics.BitLockerStatusUnavailable},
	}
	if got := formatBitLockerInventorySummary(unavailable); got != "BitLocker status unavailable (volume count unknown)" {
		t.Fatalf("unavailable summary = %q", got)
	}
}

func TestNormalizeBitLockerCmdletPayloadWithNullFieldsIsPartial(t *testing.T) {
	t.Parallel()
	// Simulated Get-BitLockerVolume JSON after null-safe conversion.
	raw := []byte(`{
		"bitlocker_volumes":[{
			"mount_point":"C:",
			"volume_status":null,
			"protection_status":"On",
			"lock_status":"Locked",
			"encryption_method":"XTS_AES_256"
		}],
		"bitlocker_inventory":{"status":"ok"}
	}`)
	var storage diagnostics.Storage
	if err := json.Unmarshal(raw, &storage); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if storage.BitLockerVolumes[0].VolumeStatus != nil {
		t.Fatalf("volume_status = %#v, want null", storage.BitLockerVolumes[0].VolumeStatus)
	}
	if storage.BitLockerVolumes[0].ProtectionStatus == nil || *storage.BitLockerVolumes[0].ProtectionStatus != "On" {
		t.Fatalf("protection_status = %#v", storage.BitLockerVolumes[0].ProtectionStatus)
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerInventory.Status != diagnostics.BitLockerStatusPartial {
		t.Fatalf("status = %q, want partial", storage.BitLockerInventory.Status)
	}
}

func TestNormalizeBitLockerCmdletPayloadCompleteRemainsOK(t *testing.T) {
	t.Parallel()
	raw := []byte(`{
		"bitlocker_volumes":[{
			"mount_point":"C:",
			"volume_status":"FullyEncrypted",
			"protection_status":"On",
			"lock_status":"Locked",
			"encryption_method":"XTS_AES_256"
		}],
		"bitlocker_inventory":{"status":"ok"}
	}`)
	var storage diagnostics.Storage
	if err := json.Unmarshal(raw, &storage); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	normalizeBitLockerInventory(&storage)
	if storage.BitLockerInventory.Status != diagnostics.BitLockerStatusOK {
		t.Fatalf("status = %q, want ok", storage.BitLockerInventory.Status)
	}
	if storage.BitLockerVolumes[0].VolumeStatus == nil || *storage.BitLockerVolumes[0].VolumeStatus != "FullyEncrypted" {
		t.Fatalf("volume_status = %#v", storage.BitLockerVolumes[0].VolumeStatus)
	}
}

func TestInventoryPowerShellDoesNotStringCastBitLockerNullFields(t *testing.T) {
	t.Parallel()
	forbidden := []string{
		"volume_status = [string]$_.VolumeStatus",
		"protection_status = [string]$_.ProtectionStatus",
		"lock_status = [string]$_.LockStatus",
		"encryption_method = [string]$_.EncryptionMethod",
	}
	for _, snippet := range forbidden {
		if strings.Contains(inventoryPowerShell, snippet) {
			t.Fatalf("inventoryPowerShell still string-casts nullable BitLocker fields: %s", snippet)
		}
	}
	if !strings.Contains(inventoryPowerShell, "Get-NullableString $volume.VolumeStatus") {
		t.Fatal("inventoryPowerShell must use Get-NullableString for VolumeStatus")
	}
}
