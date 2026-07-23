//go:build windows

package collector

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"unicode/utf16"

	"github.com/effexorxruser/EffexorWinPE/internal/diagnostics"
)

func findWindowsInstallations() []diagnostics.Installation {
	var installations []diagnostics.Installation
	for letter := 'C'; letter <= 'Z'; letter++ {
		root := fmt.Sprintf("%c:\\", letter)
		hive := filepath.Join(root, "Windows", "System32", "config", "SYSTEM")
		if info, err := os.Stat(hive); err != nil || info.IsDir() {
			continue
		}
		softwareHive := filepath.Join(root, "Windows", "System32", "config", "SOFTWARE")
		installations = append(installations, diagnostics.Installation{
			Root:         root,
			SystemHive:   hive,
			SoftwareHive: softwareHive,
			Version:      readOfflineWindowsVersion(softwareHive, letter),
		})
	}
	return filterOfflineWindowsInstallations(installations, currentRuntimeWindowsRoot(), currentEnvironmentIsWinPE(), pathExists)
}

func readOfflineWindowsVersion(softwareHive string, driveLetter rune) *diagnostics.WindowsVersion {
	if info, err := os.Stat(softwareHive); err != nil || info.IsDir() {
		return nil
	}

	mountName := fmt.Sprintf("EFFEXORWINPE_OFFLINE_%d_%c", os.Getpid(), driveLetter)
	mountPath := `HKLM\` + mountName
	if output, err := exec.Command("reg.exe", "load", mountPath, softwareHive).CombinedOutput(); err != nil {
		_ = output
		return nil
	}
	defer exec.Command("reg.exe", "unload", mountPath).Run()

	key := mountPath + `\Microsoft\Windows NT\CurrentVersion`
	read := func(name string) string {
		output, err := exec.Command("reg.exe", "query", key, "/v", name).CombinedOutput()
		if err != nil {
			return ""
		}
		return parseRegistryValue(decodeWindowsCommandOutput(output))
	}

	build := read("CurrentBuildNumber")
	if build == "" {
		build = read("CurrentBuild")
	}
	if ubr := parseRegistryInteger(read("UBR")); ubr != "" {
		if build == "" {
			build = ubr
		} else {
			build += "." + ubr
		}
	}
	rawProductName := read("ProductName")
	version := diagnostics.NormalizeWindowsVersion(diagnostics.WindowsVersion{
		RawProductName:   rawProductName,
		ProductName:      rawProductName,
		DisplayVersion:   read("DisplayVersion"),
		EditionID:        read("EditionID"),
		InstallationType: read("InstallationType"),
		Build:            build,
	})
	if version.RawProductName == "" && version.ProductName == "" && version.DisplayVersion == "" && version.EditionID == "" && version.InstallationType == "" && version.Build == "" {
		return nil
	}
	return &version
}

func decodeWindowsCommandOutput(output []byte) string {
	if len(output) >= 2 && bytes.Equal(output[:2], []byte{0xff, 0xfe}) {
		values := make([]uint16, 0, (len(output)-2)/2)
		for index := 2; index+1 < len(output); index += 2 {
			values = append(values, uint16(output[index])|uint16(output[index+1])<<8)
		}
		return string(utf16.Decode(values))
	}
	return string(output)
}

func findBCDStores() []diagnostics.BCDStore {
	stores := []diagnostics.BCDStore{}
	for letter := 'C'; letter <= 'Z'; letter++ {
		root := fmt.Sprintf("%c:\\", letter)
		candidates := []diagnostics.BCDStore{
			{Path: filepath.Join(root, "Boot", "BCD"), Kind: "bios"},
			{Path: filepath.Join(root, "EFI", "Microsoft", "Boot", "BCD"), Kind: "uefi"},
		}
		for _, candidate := range candidates {
			if info, err := os.Stat(candidate.Path); err == nil && !info.IsDir() {
				stores = append(stores, candidate)
			}
		}
	}
	return stores
}
