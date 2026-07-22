//go:build !windows

package collector

import "github.com/effexorxruser/EffexorWinPE/internal/diagnostics"

func collectPlatform() (diagnostics.Hardware, diagnostics.Storage, diagnostics.Boot, []diagnostics.Check) {
	hardware := diagnostics.Hardware{
		FirmwareMode:   "unknown",
		NetworkAdapters: []diagnostics.NetworkAdapter{},
	}
	storage := diagnostics.Storage{
		Disks:            []diagnostics.Disk{},
		DriveHealth:      []diagnostics.DriveHealth{},
		Partitions:       []diagnostics.Partition{},
		BitLockerVolumes: []diagnostics.BitLockerVolume{},
	}
	boot := diagnostics.Boot{FirmwareMode: "unknown", BCDStores: []diagnostics.BCDStore{}}
	checks := []diagnostics.Check{{
		ID:      "platform.inventory",
		Status:  "unknown",
		Summary: "Windows hardware inventory is available only when the collector runs on Windows or WinPE",
	}}
	return hardware, storage, boot, checks
}
