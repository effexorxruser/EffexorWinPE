package viewmodel

// OptionalString is a display value that may be unavailable.
type OptionalString struct {
	Value     string
	Available bool
}

// Field is a labeled display row.
type Field struct {
	LabelKey string
	Value    string
	NoteKey  string
}

// OverviewScreen is the home / system overview.
type OverviewScreen struct {
	HasReport     bool
	ReportID      string
	SchemaVersion string
	CollectedAt   string
	FirmwareMode  string
	Manufacturer  string
	Model         string
	Processor     string
	MemoryBytes   uint64
	Hostname      string
	Runtime       string
	CheckOK       int
	CheckWarning  int
	CheckError    int
	CheckUnknown  int
	MockMode      bool
	StatusMessage string
}

// ProgressScreen tracks subprocess execution.
type ProgressScreen struct {
	Phase           string // idle|collector|agent|done|failed
	StatusKey       string
	Detail          string
	Percent         int
	FriendlyError   string
	ShowJournalHint bool
}

// SummaryScreen aggregates check results.
type SummaryScreen struct {
	HasReport bool
	Headline  string
	Checks    []CheckRow
	Notes     []string
}

// CheckRow is one diagnostic check.
type CheckRow struct {
	ID        string
	StatusKey string
	Summary   string
}

// HardwareScreen shows inventory hardware.
type HardwareScreen struct {
	HasReport    bool
	FirmwareMode string
	Manufacturer string
	Model        string
	Processor    string
	Cores        uint32
	LogicalCPUs  uint32
	MemoryBytes  uint64
}

// StorageScreen shows disks, partitions, SMART.
type StorageScreen struct {
	HasReport  bool
	Disks      []DiskRow
	Health     []HealthRow
	Partitions []PartitionRow
}

// DiskRow is a storage disk.
type DiskRow struct {
	Number            int
	Name              string
	BusType           string
	SizeBytes         uint64
	HealthStatus      string
	OperationalStatus string
	IsBoot            bool
	IsSystem          bool
}

// HealthRow is SMART / reliability counters.
type HealthRow struct {
	DeviceID     string
	Name         string
	MediaType    string
	HealthStatus string
	Temperature  OptionalString
	WearPercent  OptionalString
	PowerOnHours OptionalString
	ReadErrors   OptionalString
	WriteErrors  OptionalString
}

// PartitionRow is a disk partition.
type PartitionRow struct {
	DiskNumber      int
	PartitionNumber int
	DriveLetter     string
	SizeBytes       uint64
	Type            string
	IsActive        bool
}

// BitLockerScreen shows BitLocker inventory.
type BitLockerScreen struct {
	HasReport        bool
	InventoryStatus  string // ok|partial|unavailable
	InventoryError   string
	VolumeCountKnown bool
	Volumes          []BitLockerRow
	StatusMessageKey string
}

// BitLockerRow is one BitLocker volume.
type BitLockerRow struct {
	MountPoint       string
	VolumeStatus     OptionalString
	ProtectionStatus OptionalString
	LockStatus       OptionalString
	EncryptionMethod OptionalString
}

// WindowsScreen shows offline installs.
type WindowsScreen struct {
	HasReport bool
	Installs  []WindowsRow
	EmptyKey  string
}

// WindowsRow is one Windows installation.
type WindowsRow struct {
	Root           string
	ProductName    string
	DisplayVersion string
	EditionID      string
	Build          string
	InstallType    string
}

// NetworkScreen shows adapters / Ethernet.
type NetworkScreen struct {
	HasReport         bool
	Adapters          []NetworkRow
	EthernetConnected bool
	StatusMessageKey  string
}

// NetworkRow is one NIC.
type NetworkRow struct {
	Name        string
	Description string
	StatusKey   string
	StatusRaw   string
	Connected   bool
}

// AgentScreen shows assessment results.
type AgentScreen struct {
	HasAssessment bool
	Mode          string
	Headline      string
	Severity      string
	FindingCount  int
	Findings      []FindingRow
	NextSteps     []NextStepRow
	Limitations   []string
	SessionID     string
}

// FindingRow is one assessment finding.
type FindingRow struct {
	ID         string
	Title      string
	Severity   string
	Confidence string
	Rationale  string
}

// NextStepRow is a read-only next step.
type NextStepRow struct {
	ID        string
	Title     string
	Operation string
	Risk      string
	Rationale string
}

// ExportScreen is export UI state.
type ExportScreen struct {
	ReportPath    string
	DiagnosisPath string
	SessionPath   string
	JournalPath   string
	TargetDir     string
	LastMessage   string
	LastOK        bool
}

// JournalScreen is the log view.
type JournalScreen struct {
	Entries []string
}

// AppModel is the full shell presentation model.
type AppModel struct {
	MockMode  bool
	Locale    string
	Overview  OverviewScreen
	Progress  ProgressScreen
	Summary   SummaryScreen
	Hardware  HardwareScreen
	Storage   StorageScreen
	BitLocker BitLockerScreen
	Windows   WindowsScreen
	Network   NetworkScreen
	Agent     AgentScreen
	Export    ExportScreen
	Journal   JournalScreen
}
