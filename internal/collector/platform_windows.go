//go:build windows

package collector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

type inventoryPayload struct {
	Hardware diagnostics.Hardware `json:"hardware"`
	Storage  diagnostics.Storage  `json:"storage"`
	Errors   []string             `json:"errors"`
}

func collectPlatform() (diagnostics.Hardware, diagnostics.Storage, diagnostics.Boot, []diagnostics.Check) {
	payload, err := runPowerShellInventory()
	if err != nil {
		hardware, storage, boot := emptyPlatformReport()
		return hardware, storage, boot, []diagnostics.Check{{
			ID:      "platform.inventory",
			Status:  "error",
			Summary: err.Error(),
		}}
	}

	normalizeInventory(&payload)
	boot := diagnostics.Boot{
		FirmwareMode: payload.Hardware.FirmwareMode,
		BCDStores:    findBCDStores(),
	}
	checks := buildPlatformChecks(payload.Storage, len(payload.Hardware.NetworkAdapters), len(boot.BCDStores), payload.Errors)
	return payload.Hardware, payload.Storage, boot, checks
}

func runPowerShellInventory() (inventoryPayload, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	command := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", inventoryPowerShell)
	raw, err := command.Output()
	if ctx.Err() == context.DeadlineExceeded {
		return inventoryPayload{}, fmt.Errorf("Windows inventory timed out after 30 seconds")
	}
	if err != nil {
		if exitError, ok := err.(*exec.ExitError); ok {
			return inventoryPayload{}, fmt.Errorf("PowerShell inventory failed: %s", strings.TrimSpace(string(exitError.Stderr)))
		}
		return inventoryPayload{}, fmt.Errorf("start PowerShell inventory: %w", err)
	}

	raw = bytes.TrimPrefix(raw, []byte{0xef, 0xbb, 0xbf})
	var payload inventoryPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return inventoryPayload{}, fmt.Errorf("decode PowerShell inventory: %w", err)
	}
	return payload, nil
}

func emptyPlatformReport() (diagnostics.Hardware, diagnostics.Storage, diagnostics.Boot) {
	hardware := diagnostics.Hardware{FirmwareMode: "unknown", NetworkAdapters: []diagnostics.NetworkAdapter{}}
	storage := diagnostics.Storage{
		Disks:            []diagnostics.Disk{},
		DriveHealth:      []diagnostics.DriveHealth{},
		Partitions:       []diagnostics.Partition{},
		BitLockerVolumes: nil,
		BitLockerInventory: diagnostics.BitLockerInventory{
			Status: diagnostics.BitLockerStatusUnavailable,
			Error:  "platform inventory failed before BitLocker collection",
		},
	}
	boot := diagnostics.Boot{FirmwareMode: "unknown", BCDStores: findBCDStores()}
	return hardware, storage, boot
}

func normalizeInventory(payload *inventoryPayload) {
	if payload.Hardware.FirmwareMode == "" {
		payload.Hardware.FirmwareMode = "unknown"
	}
	if payload.Hardware.NetworkAdapters == nil {
		payload.Hardware.NetworkAdapters = []diagnostics.NetworkAdapter{}
	}
	for index := range payload.Hardware.NetworkAdapters {
		payload.Hardware.NetworkAdapters[index] = diagnostics.NormalizeNetworkAdapter(payload.Hardware.NetworkAdapters[index])
	}
	if payload.Storage.Disks == nil {
		payload.Storage.Disks = []diagnostics.Disk{}
	}
	if payload.Storage.DriveHealth == nil {
		payload.Storage.DriveHealth = []diagnostics.DriveHealth{}
	}
	if payload.Storage.Partitions == nil {
		payload.Storage.Partitions = []diagnostics.Partition{}
	}
	normalizeBitLockerInventory(&payload.Storage)
	if payload.Errors == nil {
		payload.Errors = []string{}
	}
}
