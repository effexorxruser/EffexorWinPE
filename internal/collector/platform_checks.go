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
	if check := bitLockerInventoryCheck(storage.BitLockerInventory); check != nil {
		checks = append(checks, *check)
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

func bitLockerInventoryCheck(inventory diagnostics.BitLockerInventory) *diagnostics.Check {
	switch inventory.Status {
	case diagnostics.BitLockerStatusUnavailable:
		summary := "BitLocker inventory is unavailable"
		if inventory.Error != "" {
			summary = "BitLocker inventory is unavailable: " + inventory.Error
		}
		return &diagnostics.Check{ID: "storage.bitlocker_inventory", Status: "unknown", Summary: summary}
	case diagnostics.BitLockerStatusPartial:
		summary := "BitLocker inventory is partial"
		if inventory.Error != "" {
			summary = "BitLocker inventory is partial: " + inventory.Error
		}
		return &diagnostics.Check{ID: "storage.bitlocker_inventory", Status: "unknown", Summary: summary}
	default:
		return nil
	}
}

func filterBitLockerSourceErrors(sourceErrors []string, inventory diagnostics.BitLockerInventory) []string {
	if len(sourceErrors) == 0 {
		return []string{}
	}
	filtered := make([]string, 0, len(sourceErrors))
	for _, sourceError := range sourceErrors {
		if isBitLockerSourceError(sourceError) &&
			(inventory.Status == diagnostics.BitLockerStatusUnavailable || inventory.Status == diagnostics.BitLockerStatusPartial) {
			continue
		}
		filtered = append(filtered, sourceError)
	}
	return filtered
}

func isBitLockerSourceError(summary string) bool {
	lower := strings.ToLower(summary)
	return strings.Contains(lower, "bitlocker inventory is unavailable") ||
		strings.Contains(lower, "bitlocker inventory is partial")
}
