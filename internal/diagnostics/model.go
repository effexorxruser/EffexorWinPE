package diagnostics

import "time"

const SchemaVersion = "1.3.0"

type Report struct {
	SchemaVersion string         `json:"schema_version"`
	ReportID      string         `json:"report_id"`
	CollectedAt   time.Time      `json:"collected_at"`
	Collector     Collector      `json:"collector"`
	Environment   Environment    `json:"environment"`
	Hardware      Hardware       `json:"hardware"`
	Storage       Storage        `json:"storage"`
	Boot          Boot           `json:"boot"`
	Installations []Installation `json:"windows_installations"`
	Checks        []Check        `json:"checks"`
	Privacy       Privacy        `json:"privacy"`
}

type Collector struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Environment struct {
	RuntimeOS   string `json:"runtime_os"`
	RuntimeArch string `json:"runtime_arch"`
	Hostname    string `json:"hostname,omitempty"`
}

type Hardware struct {
	FirmwareMode    string           `json:"firmware_mode"`
	System          System           `json:"system"`
	Processor       Processor        `json:"processor"`
	Memory          Memory           `json:"memory"`
	NetworkAdapters []NetworkAdapter `json:"network_adapters"`
}

type System struct {
	Manufacturer string `json:"manufacturer,omitempty"`
	Model        string `json:"model,omitempty"`
}

type Processor struct {
	Name              string `json:"name,omitempty"`
	Cores             uint32 `json:"cores"`
	LogicalProcessors uint32 `json:"logical_processors"`
}

type Memory struct {
	TotalPhysicalBytes uint64 `json:"total_physical_bytes"`
}

// NetworkAdapter describes a physical NIC.
// Status is a stable machine-readable enum derived from Win32_NetworkAdapter.NetConnectionStatus.
// StatusCode preserves the raw numeric provider value when available, including unknown codes.
type NetworkAdapter struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Status      string `json:"status,omitempty"`
	StatusCode  *int   `json:"status_code,omitempty"`
}

// BitLocker inventory availability values.
const (
	BitLockerStatusOK          = "ok"
	BitLockerStatusUnavailable = "unavailable"
	BitLockerStatusPartial     = "partial"
)

type Storage struct {
	Disks              []Disk             `json:"disks"`
	DriveHealth        []DriveHealth      `json:"drive_health"`
	Partitions         []Partition        `json:"partitions"`
	BitLockerVolumes   []BitLockerVolume  `json:"bitlocker_volumes"`
	BitLockerInventory BitLockerInventory `json:"bitlocker_inventory"`
}

// BitLockerInventory records whether the BitLocker provider was successfully queried.
// An empty bitlocker_volumes array is meaningful only when Status is "ok" or "partial".
// When Status is "unavailable", bitlocker_volumes must be null rather than [].
type BitLockerInventory struct {
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type DriveHealth struct {
	DeviceID         string  `json:"device_id"`
	FriendlyName     string  `json:"friendly_name,omitempty"`
	MediaType        string  `json:"media_type,omitempty"`
	HealthStatus     string  `json:"health_status,omitempty"`
	TemperatureC     *uint64 `json:"temperature_celsius"`
	WearPercent      *uint64 `json:"wear_percent"`
	PowerOnHours     *uint64 `json:"power_on_hours"`
	ReadErrorsTotal  *uint64 `json:"read_errors_total"`
	WriteErrorsTotal *uint64 `json:"write_errors_total"`
}

type Disk struct {
	Number            int    `json:"number"`
	FriendlyName      string `json:"friendly_name,omitempty"`
	BusType           string `json:"bus_type,omitempty"`
	SizeBytes         uint64 `json:"size_bytes"`
	PartitionStyle    string `json:"partition_style,omitempty"`
	HealthStatus      string `json:"health_status,omitempty"`
	OperationalStatus string `json:"operational_status,omitempty"`
	IsBoot            bool   `json:"is_boot"`
	IsSystem          bool   `json:"is_system"`
}

type Partition struct {
	DiskNumber      int    `json:"disk_number"`
	PartitionNumber int    `json:"partition_number"`
	DriveLetter     string `json:"drive_letter,omitempty"`
	SizeBytes       uint64 `json:"size_bytes"`
	Type            string `json:"type,omitempty"`
	GPTType         string `json:"gpt_type,omitempty"`
	IsActive        bool   `json:"is_active"`
}

type BitLockerVolume struct {
	MountPoint       string `json:"mount_point"`
	VolumeStatus     string `json:"volume_status,omitempty"`
	ProtectionStatus string `json:"protection_status,omitempty"`
	LockStatus       string `json:"lock_status,omitempty"`
	EncryptionMethod string `json:"encryption_method,omitempty"`
}

type Boot struct {
	FirmwareMode string     `json:"firmware_mode"`
	BCDStores    []BCDStore `json:"bcd_stores"`
}

type BCDStore struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
}

type Installation struct {
	Root         string          `json:"root"`
	SystemHive   string          `json:"system_hive"`
	SoftwareHive string          `json:"software_hive"`
	Version      *WindowsVersion `json:"version,omitempty"`
}

type WindowsVersion struct {
	RawProductName   string `json:"raw_product_name,omitempty"`
	ProductName      string `json:"product_name,omitempty"`
	DisplayVersion   string `json:"display_version,omitempty"`
	EditionID        string `json:"edition_id,omitempty"`
	InstallationType string `json:"installation_type,omitempty"`
	Build            string `json:"build,omitempty"`
}

type Check struct {
	ID      string `json:"id"`
	Status  string `json:"status"`
	Summary string `json:"summary"`
}

type Privacy struct {
	ContainsPersonalData bool     `json:"contains_personal_data"`
	ExcludedByDefault    []string `json:"excluded_by_default"`
}
