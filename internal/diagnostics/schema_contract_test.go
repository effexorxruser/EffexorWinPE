package diagnostics_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestDiagnosticReportSchemaBitLockerConditionals(t *testing.T) {
	t.Parallel()
	schema := compileReportSchema(t)

	baseReport := func(storage map[string]any) map[string]any {
		return map[string]any{
			"schema_version": "1.3.0",
			"report_id":      "report000000000001",
			"collected_at":   "2026-07-22T12:00:00Z",
			"collector":      map[string]any{"name": "effexorwinpe-collector", "version": "test"},
			"environment":    map[string]any{"runtime_os": "windows", "runtime_arch": "amd64"},
			"hardware": map[string]any{
				"firmware_mode":    "uefi",
				"system":           map[string]any{},
				"processor":        map[string]any{"cores": 0, "logical_processors": 0},
				"memory":           map[string]any{"total_physical_bytes": 0},
				"network_adapters": []any{},
			},
			"storage":               storage,
			"boot":                  map[string]any{"firmware_mode": "uefi", "bcd_stores": []any{}},
			"windows_installations": []any{},
			"checks":                []any{},
			"privacy": map[string]any{
				"contains_personal_data": false,
				"excluded_by_default":    []any{},
			},
		}
	}

	tests := []struct {
		name    string
		storage map[string]any
		wantOK  bool
	}{
		{
			name: "unavailable with null volumes",
			storage: map[string]any{
				"disks": []any{}, "drive_health": []any{}, "partitions": []any{},
				"bitlocker_volumes":   nil,
				"bitlocker_inventory": map[string]any{"status": "unavailable"},
			},
			wantOK: true,
		},
		{
			name: "ok with empty array",
			storage: map[string]any{
				"disks": []any{}, "drive_health": []any{}, "partitions": []any{},
				"bitlocker_volumes":   []any{},
				"bitlocker_inventory": map[string]any{"status": "ok"},
			},
			wantOK: true,
		},
		{
			name: "partial with explicit null fields",
			storage: map[string]any{
				"disks": []any{}, "drive_health": []any{}, "partitions": []any{},
				"bitlocker_volumes": []any{map[string]any{
					"mount_point": "C:", "volume_status": nil, "protection_status": nil,
					"lock_status": nil, "encryption_method": nil,
				}},
				"bitlocker_inventory": map[string]any{"status": "partial", "error": "incomplete"},
			},
			wantOK: true,
		},
		{
			name: "unavailable with empty array",
			storage: map[string]any{
				"disks": []any{}, "drive_health": []any{}, "partitions": []any{},
				"bitlocker_volumes":   []any{},
				"bitlocker_inventory": map[string]any{"status": "unavailable"},
			},
			wantOK: false,
		},
		{
			name: "ok with null volumes",
			storage: map[string]any{
				"disks": []any{}, "drive_health": []any{}, "partitions": []any{},
				"bitlocker_volumes":   nil,
				"bitlocker_inventory": map[string]any{"status": "ok"},
			},
			wantOK: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			raw, err := json.Marshal(baseReport(test.storage))
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var instance any
			if err := json.Unmarshal(raw, &instance); err != nil {
				t.Fatalf("Unmarshal instance: %v", err)
			}
			err = schema.Validate(instance)
			if test.wantOK && err != nil {
				t.Fatalf("schema.Validate() error = %v", err)
			}
			if !test.wantOK && err == nil {
				t.Fatal("schema.Validate() error = nil, want failure")
			}
		})
	}
}

func compileReportSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	path := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "contracts", "diagnostic-report.schema.json"))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource("https://effexorwinpe.local/contracts/diagnostic-report.schema.json", bytes.NewReader(raw)); err != nil {
		t.Fatalf("AddResource() error = %v", err)
	}
	schema, err := compiler.Compile("https://effexorwinpe.local/contracts/diagnostic-report.schema.json")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return schema
}
