package diagnostics_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDiagnosticReportSchemaDefinesBitLockerConditionals(t *testing.T) {
	t.Parallel()
	schema := loadSchema(t)
	storage, ok := schema["$defs"].(map[string]any)["storage"].(map[string]any)
	if !ok {
		t.Fatal("schema is missing $defs.storage")
	}
	allOf, ok := storage["allOf"].([]any)
	if !ok || len(allOf) < 2 {
		t.Fatalf("storage.allOf = %#v, want at least two conditional rules", storage["allOf"])
	}
	raw, err := json.Marshal(allOf)
	if err != nil {
		t.Fatalf("marshal allOf: %v", err)
	}
	text := string(raw)
	for _, part := range []string{`"const":"unavailable"`, `"type":"null"`, `"enum":["ok","partial"]`, `"type":"array"`} {
		if !strings.Contains(text, part) {
			t.Fatalf("storage.allOf is missing %s: %s", part, text)
		}
	}
}

func TestBitLockerSchemaConditionalsPositiveAndNegative(t *testing.T) {
	t.Parallel()
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
			name: "partial with volume",
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
			err := validateStorageBitLockerConditionals(test.storage)
			if test.wantOK && err != nil {
				t.Fatalf("validateStorageBitLockerConditionals() error = %v", err)
			}
			if !test.wantOK && err == nil {
				t.Fatal("validateStorageBitLockerConditionals() error = nil, want failure")
			}
		})
	}
}

func validateStorageBitLockerConditionals(storage map[string]any) error {
	inventory, _ := storage["bitlocker_inventory"].(map[string]any)
	status, _ := inventory["status"].(string)
	volumes, hasVolumes := storage["bitlocker_volumes"]
	switch status {
	case "unavailable":
		if hasVolumes && volumes != nil {
			return errString("bitlocker_volumes must be null when status is unavailable")
		}
	case "ok", "partial":
		if !hasVolumes || volumes == nil {
			return errString("bitlocker_volumes must be an array when status is ok or partial")
		}
		if _, ok := volumes.([]any); !ok {
			return errString("bitlocker_volumes must be an array when status is ok or partial")
		}
	default:
		return errString("invalid bitlocker_inventory.status")
	}
	return nil
}

type errString string

func (err errString) Error() string { return string(err) }

func loadSchema(t *testing.T) map[string]any {
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
	var schema map[string]any
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatalf("decode schema: %v", err)
	}
	return schema
}
