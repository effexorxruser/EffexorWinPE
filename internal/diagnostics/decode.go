package diagnostics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Supported legacy diagnostic report schema version.
const SchemaVersionV12 = "1.2.0"

// DecodeReportJSON decodes a diagnostic report. Schema 1.3.0 is validated
// strictly. Schema 1.2.0 is accepted only through an explicit migration that
// never treats a legacy empty bitlocker_volumes array as a confirmed "ok".
func DecodeReportJSON(data []byte) (Report, error) {
	var probe struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return Report{}, fmt.Errorf("decode diagnostic report: %w", err)
	}
	switch strings.TrimSpace(probe.SchemaVersion) {
	case SchemaVersion:
		return decodeReportV13(data)
	case SchemaVersionV12:
		return migrateReportV12(data)
	case "":
		return Report{}, fmt.Errorf("diagnostic report schema_version is required")
	default:
		return Report{}, fmt.Errorf("unsupported diagnostic schema %q", probe.SchemaVersion)
	}
}

// DecodeReport reads and decodes a diagnostic report from r.
func DecodeReport(r io.Reader) (Report, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Report{}, err
	}
	return DecodeReportJSON(data)
}

// UnmarshalJSON decodes current and legacy diagnostic reports through DecodeReportJSON.
func (r *Report) UnmarshalJSON(data []byte) error {
	decoded, err := DecodeReportJSON(data)
	if err != nil {
		return err
	}
	*r = decoded
	return nil
}

func decodeReportV13(data []byte) (Report, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	// Avoid recursing into Report.UnmarshalJSON.
	type reportPlain Report
	var plain reportPlain
	if err := decoder.Decode(&plain); err != nil {
		return Report{}, fmt.Errorf("decode diagnostic report 1.3.0: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Report{}, err
	}
	report := Report(plain)
	if report.SchemaVersion != SchemaVersion {
		return Report{}, fmt.Errorf("unsupported diagnostic schema %q", report.SchemaVersion)
	}
	if err := ValidateBitLockerContract(report.Storage); err != nil {
		return Report{}, err
	}
	if err := validateRequiredNullableFields(data); err != nil {
		return Report{}, err
	}
	normalizeDecodedReport(&report)
	return report, nil
}

type reportV12DTO struct {
	SchemaVersion string            `json:"schema_version"`
	ReportID      string            `json:"report_id"`
	CollectedAt   time.Time         `json:"collected_at"`
	Collector     Collector         `json:"collector"`
	Environment   Environment       `json:"environment"`
	Hardware      hardwareV12DTO    `json:"hardware"`
	Storage       storageV12DTO     `json:"storage"`
	Boot          Boot              `json:"boot"`
	Installations []installationV12 `json:"windows_installations"`
	Checks        []Check           `json:"checks"`
	Privacy       Privacy           `json:"privacy"`
}

type hardwareV12DTO struct {
	FirmwareMode    string              `json:"firmware_mode"`
	System          System              `json:"system"`
	Processor       Processor           `json:"processor"`
	Memory          Memory              `json:"memory"`
	NetworkAdapters []networkAdapterV12 `json:"network_adapters"`
}

type networkAdapterV12 struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
}

type storageV12DTO struct {
	Disks            []Disk               `json:"disks"`
	DriveHealth      []DriveHealth        `json:"drive_health"`
	Partitions       []Partition          `json:"partitions"`
	BitLockerVolumes []bitLockerVolumeV12 `json:"bitlocker_volumes"`
}

type bitLockerVolumeV12 struct {
	MountPoint       string `json:"mount_point"`
	VolumeStatus     string `json:"volume_status,omitempty"`
	ProtectionStatus string `json:"protection_status,omitempty"`
	LockStatus       string `json:"lock_status,omitempty"`
	EncryptionMethod string `json:"encryption_method,omitempty"`
}

type installationV12 struct {
	Root         string             `json:"root"`
	SystemHive   string             `json:"system_hive"`
	SoftwareHive string             `json:"software_hive"`
	Version      *windowsVersionV12 `json:"version,omitempty"`
}

type windowsVersionV12 struct {
	ProductName      string `json:"product_name,omitempty"`
	DisplayVersion   string `json:"display_version,omitempty"`
	EditionID        string `json:"edition_id,omitempty"`
	InstallationType string `json:"installation_type,omitempty"`
	Build            string `json:"build,omitempty"`
}

func migrateReportV12(data []byte) (Report, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var legacy reportV12DTO
	if err := decoder.Decode(&legacy); err != nil {
		return Report{}, fmt.Errorf("decode diagnostic report 1.2.0: %w", err)
	}
	if err := requireJSONEOF(decoder); err != nil {
		return Report{}, err
	}
	if legacy.SchemaVersion != SchemaVersionV12 {
		return Report{}, fmt.Errorf("unsupported diagnostic schema %q", legacy.SchemaVersion)
	}

	report := Report{
		SchemaVersion: SchemaVersion,
		ReportID:      legacy.ReportID,
		CollectedAt:   legacy.CollectedAt,
		Collector:     legacy.Collector,
		Environment:   legacy.Environment,
		Hardware: Hardware{
			FirmwareMode: legacy.Hardware.FirmwareMode,
			System:       legacy.Hardware.System,
			Processor:    legacy.Hardware.Processor,
			Memory:       legacy.Hardware.Memory,
		},
		Boot:    legacy.Boot,
		Checks:  legacy.Checks,
		Privacy: legacy.Privacy,
	}

	for _, adapter := range legacy.Hardware.NetworkAdapters {
		report.Hardware.NetworkAdapters = append(report.Hardware.NetworkAdapters, NormalizeNetworkAdapter(NetworkAdapter{
			Name:        adapter.Name,
			Description: adapter.Description,
			Status:      adapter.Status,
		}))
	}
	if report.Hardware.NetworkAdapters == nil {
		report.Hardware.NetworkAdapters = []NetworkAdapter{}
	}

	report.Storage.Disks = legacy.Storage.Disks
	report.Storage.DriveHealth = legacy.Storage.DriveHealth
	report.Storage.Partitions = legacy.Storage.Partitions
	if report.Storage.Disks == nil {
		report.Storage.Disks = []Disk{}
	}
	if report.Storage.DriveHealth == nil {
		report.Storage.DriveHealth = []DriveHealth{}
	}
	if report.Storage.Partitions == nil {
		report.Storage.Partitions = []Partition{}
	}
	report.Storage.BitLockerInventory, report.Storage.BitLockerVolumes = migrateBitLockerV12(legacy.Storage.BitLockerVolumes)

	for _, installation := range legacy.Installations {
		migrated := Installation{
			Root:         installation.Root,
			SystemHive:   installation.SystemHive,
			SoftwareHive: installation.SoftwareHive,
		}
		if installation.Version != nil {
			version := NormalizeWindowsVersion(WindowsVersion{
				RawProductName:   installation.Version.ProductName,
				ProductName:      installation.Version.ProductName,
				DisplayVersion:   installation.Version.DisplayVersion,
				EditionID:        installation.Version.EditionID,
				InstallationType: installation.Version.InstallationType,
				Build:            installation.Version.Build,
			})
			migrated.Version = &version
		}
		report.Installations = append(report.Installations, migrated)
	}
	if report.Installations == nil {
		report.Installations = []Installation{}
	}
	if report.Checks == nil {
		report.Checks = []Check{}
	}
	if report.Boot.BCDStores == nil {
		report.Boot.BCDStores = []BCDStore{}
	}
	if err := ValidateBitLockerContract(report.Storage); err != nil {
		return Report{}, err
	}
	normalizeDecodedReport(&report)
	return report, nil
}

func migrateBitLockerV12(volumes []bitLockerVolumeV12) (BitLockerInventory, []BitLockerVolume) {
	// Legacy 1.2.0 always serialized an array. An empty array cannot be trusted as
	// "provider OK, no volumes" because provider failures were also written as [].
	if len(volumes) == 0 {
		return BitLockerInventory{
			Status: BitLockerStatusUnavailable,
			Error:  "migrated from schema 1.2.0 without BitLocker availability signal; empty bitlocker_volumes is treated as unavailable",
		}, nil
	}
	migrated := make([]BitLockerVolume, 0, len(volumes))
	for _, volume := range volumes {
		migrated = append(migrated, BitLockerVolume{
			MountPoint:       volume.MountPoint,
			VolumeStatus:     nonEmptyStringPointer(volume.VolumeStatus),
			ProtectionStatus: nonEmptyStringPointer(volume.ProtectionStatus),
			LockStatus:       nonEmptyStringPointer(volume.LockStatus),
			EncryptionMethod: nonEmptyStringPointer(volume.EncryptionMethod),
		})
	}
	return BitLockerInventory{Status: BitLockerStatusOK}, migrated
}

func nonEmptyStringPointer(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func normalizeDecodedReport(report *Report) {
	for index := range report.Hardware.NetworkAdapters {
		report.Hardware.NetworkAdapters[index] = NormalizeNetworkAdapter(report.Hardware.NetworkAdapters[index])
	}
	for index := range report.Installations {
		if report.Installations[index].Version != nil {
			normalized := NormalizeWindowsVersion(*report.Installations[index].Version)
			report.Installations[index].Version = &normalized
		}
	}
}

// ValidateBitLockerContract enforces the 1.3.0 availability/volumes invariants.
func ValidateBitLockerContract(storage Storage) error {
	switch storage.BitLockerInventory.Status {
	case BitLockerStatusUnavailable:
		if storage.BitLockerVolumes != nil {
			return fmt.Errorf("bitlocker_volumes must be null when bitlocker_inventory.status is unavailable")
		}
	case BitLockerStatusOK, BitLockerStatusPartial:
		if storage.BitLockerVolumes == nil {
			return fmt.Errorf("bitlocker_volumes must be an array when bitlocker_inventory.status is %s", storage.BitLockerInventory.Status)
		}
	default:
		return fmt.Errorf("invalid bitlocker_inventory.status %q", storage.BitLockerInventory.Status)
	}
	return nil
}

func requireJSONEOF(decoder *json.Decoder) error {
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("diagnostic report contains trailing JSON data")
		}
		return err
	}
	return nil
}

var (
	requiredDriveHealthNullableFields = []string{
		"temperature_celsius",
		"wear_percent",
		"power_on_hours",
		"read_errors_total",
		"write_errors_total",
	}
	requiredBitLockerVolumeNullableFields = []string{
		"volume_status",
		"protection_status",
		"lock_status",
		"encryption_method",
	}
)

// validateRequiredNullableFields ensures 1.3.0 reports explicitly include nullable
// metric/status keys (JSON null is allowed; omission is not).
func validateRequiredNullableFields(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return err
	}
	storageRaw, ok := root["storage"]
	if !ok {
		return fmt.Errorf("storage is required")
	}
	var storage map[string]json.RawMessage
	if err := json.Unmarshal(storageRaw, &storage); err != nil {
		return fmt.Errorf("decode storage: %w", err)
	}
	if err := validateObjectArrayNullableFields(storage["drive_health"], "storage.drive_health", requiredDriveHealthNullableFields); err != nil {
		return err
	}
	volumesRaw, ok := storage["bitlocker_volumes"]
	if !ok {
		return fmt.Errorf("storage.bitlocker_volumes is required")
	}
	if string(volumesRaw) == "null" {
		return nil
	}
	return validateObjectArrayNullableFields(volumesRaw, "storage.bitlocker_volumes", requiredBitLockerVolumeNullableFields)
}

func validateObjectArrayNullableFields(raw json.RawMessage, path string, required []string) error {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	for index, item := range items {
		for _, field := range required {
			if _, ok := item[field]; !ok {
				return fmt.Errorf("%s[%d].%s is required and may be null, but must not be omitted", path, index, field)
			}
		}
	}
	return nil
}
