package collector

import (
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
