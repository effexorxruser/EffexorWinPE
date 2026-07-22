package collector

import (
	"strings"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func normalizeBitLockerInventory(storage *diagnostics.Storage) {
	status := strings.TrimSpace(strings.ToLower(storage.BitLockerInventory.Status))
	switch status {
	case diagnostics.BitLockerStatusOK, diagnostics.BitLockerStatusPartial, diagnostics.BitLockerStatusUnavailable:
		storage.BitLockerInventory.Status = status
	case "":
		if storage.BitLockerVolumes == nil {
			storage.BitLockerInventory.Status = diagnostics.BitLockerStatusUnavailable
			if storage.BitLockerInventory.Error == "" {
				storage.BitLockerInventory.Error = "BitLocker inventory status was not reported"
			}
		} else {
			storage.BitLockerInventory.Status = diagnostics.BitLockerStatusOK
		}
	default:
		storage.BitLockerInventory.Status = diagnostics.BitLockerStatusUnavailable
		if storage.BitLockerInventory.Error == "" {
			storage.BitLockerInventory.Error = "unexpected BitLocker inventory status: " + status
		}
		storage.BitLockerVolumes = nil
		return
	}

	switch storage.BitLockerInventory.Status {
	case diagnostics.BitLockerStatusUnavailable:
		storage.BitLockerVolumes = nil
	case diagnostics.BitLockerStatusOK, diagnostics.BitLockerStatusPartial:
		if storage.BitLockerVolumes == nil {
			storage.BitLockerVolumes = []diagnostics.BitLockerVolume{}
		}
	}
}
