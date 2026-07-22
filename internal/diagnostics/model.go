package diagnostics

import "time"

const SchemaVersion = "1.0.0"

type Report struct {
	SchemaVersion string         `json:"schema_version"`
	ReportID      string         `json:"report_id"`
	CollectedAt   time.Time      `json:"collected_at"`
	Collector     Collector      `json:"collector"`
	Environment   Environment    `json:"environment"`
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

type Installation struct {
	Root       string `json:"root"`
	SystemHive string `json:"system_hive"`
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
