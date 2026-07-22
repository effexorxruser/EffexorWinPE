package collector

import (
	"fmt"
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func buildPlatformChecks(storage diagnostics.Storage, networkAdapters int, bcdStores int, sourceErrors []string) []diagnostics.Check {
	filteredErrors := filterBitLockerSourceErrors(sourceErrors, storage.BitLockerInventory)
	inventoryStatus := "ok"
	inventorySuffix := ""
	if len(filteredErrors) > 0 {
		inventoryStatus = "warning"
		inventorySuffix = fmt.Sprintf("; %d source(s) unavailable", len(filteredErrors))
	}

	bitLockerSummary := formatBitLockerInventorySummary(storage)
	checks := []diagnostics.Check{{
		ID:      "platform.inventory",
		Status:  inventoryStatus,
		Summary: fmt.Sprintf("Collected %d disk(s), %d drive-health record(s), %d partition(s), %d network adapter(s), and %s%s", len(storage.Disks), len(storage.DriveHealth), len(storage.Partitions), networkAdapters, bitLockerSummary, inventorySuffix),
	}}
	for index, sourceError := range filteredErrors {
		checks = append(checks, diagnostics.Check{
			ID:      fmt.Sprintf("platform.inventory.source.%d", index+1),
			Status:  "unknown",
			Summary: sourceError,
		})
	}
	if storage.BitLockerInventory.Status == diagnostics.BitLockerStatusUnavailable {
		summary := "BitLocker inventory is unavailable"
		if storage.BitLockerInventory.Error != "" {
			summary = "BitLocker inventory is unavailable: " + storage.BitLockerInventory.Error
		}
		checks = append(checks, diagnostics.Check{
			ID:      "storage.bitlocker_inventory",
			Status:  "unknown",
			Summary: summary,
		})
	}
	if bcdStores == 0 {
		checks = append(checks, diagnostics.Check{
			ID:      "boot.bcd_stores",
			Status:  "warning",
			Summary: "No BCD store was found on a mounted volume",
		})
	} else {
		checks = append(checks, diagnostics.Check{
			ID:      "boot.bcd_stores",
			Status:  "ok",
			Summary: fmt.Sprintf("Found %d BCD store(s) on mounted volumes", bcdStores),
		})
	}
	return checks
}

func formatBitLockerInventorySummary(storage diagnostics.Storage) string {
	status := storage.BitLockerInventory.Status
	switch status {
	case diagnostics.BitLockerStatusUnavailable:
		return fmt.Sprintf("BitLocker status %s (volume count unknown)", status)
	case diagnostics.BitLockerStatusOK, diagnostics.BitLockerStatusPartial:
		count := 0
		if storage.BitLockerVolumes != nil {
			count = len(storage.BitLockerVolumes)
		}
		return fmt.Sprintf("BitLocker status %s (%d volume(s))", status, count)
	default:
		return fmt.Sprintf("BitLocker status %s (volume count unknown)", status)
	}
}

func filterBitLockerSourceErrors(sourceErrors []string, inventory diagnostics.BitLockerInventory) []string {
	if len(sourceErrors) == 0 {
		return []string{}
	}
	filtered := make([]string, 0, len(sourceErrors))
	for _, sourceError := range sourceErrors {
		if inventory.Status == diagnostics.BitLockerStatusUnavailable && isBitLockerSourceError(sourceError) {
			continue
		}
		filtered = append(filtered, sourceError)
	}
	return filtered
}

func isBitLockerSourceError(summary string) bool {
	return strings.Contains(strings.ToLower(summary), "bitlocker inventory is unavailable")
}
