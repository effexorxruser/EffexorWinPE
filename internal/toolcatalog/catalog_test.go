package toolcatalog_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

const schemaURL = "https://effexorwinpe.local/manifests/tool-catalog.schema.json"

func TestToolCatalogJSONMatchesSchema(t *testing.T) {
	t.Parallel()
	schema := compileCatalogSchema(t)
	raw := readRepoFile(t, "manifests", "tool-catalog.json")

	var instance any
	if err := json.Unmarshal(raw, &instance); err != nil {
		t.Fatalf("Unmarshal catalog: %v", err)
	}
	if err := schema.Validate(instance); err != nil {
		t.Fatalf("schema.Validate(tool-catalog.json) error = %v", err)
	}
}

func TestToolCatalogProfileToolIDsExist(t *testing.T) {
	t.Parallel()
	catalog := loadCatalog(t)

	ids := make(map[string]struct{}, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		if _, exists := ids[tool.ID]; exists {
			t.Fatalf("duplicate tool id %q", tool.ID)
		}
		ids[tool.ID] = struct{}{}
	}

	requiredProfiles := map[string]struct{}{
		"minimal-diagnostics": {},
		"technician-standard": {},
		"data-recovery":       {},
		"network-enabled":     {},
		"multiboot-extras":    {},
	}
	for _, profile := range catalog.Profiles {
		delete(requiredProfiles, profile.ID)
		for _, toolID := range profile.ToolIDs {
			if _, ok := ids[toolID]; !ok {
				t.Fatalf("profile %q references unknown tool id %q", profile.ID, toolID)
			}
		}
	}
	for missing := range requiredProfiles {
		t.Fatalf("missing required release profile %q", missing)
	}
}

func TestToolCatalogRequiredFieldsPresent(t *testing.T) {
	t.Parallel()
	catalog := loadCatalog(t)
	if catalog.SchemaVersion != 1 {
		t.Fatalf("schema_version = %d, want 1", catalog.SchemaVersion)
	}
	if len(catalog.Tools) == 0 {
		t.Fatal("tools must not be empty")
	}

	requiredCategories := map[string]bool{
		"disks_and_partitions":      false,
		"backup_and_restore":        false,
		"data_recovery":             false,
		"windows_boot_repair":       false,
		"hardware_diagnostics":      false,
		"network":                   false,
		"malware_scanning":          false,
		"windows_installation":      false,
		"password_account_recovery": false,
		"firmware_and_uefi":         false,
		"file_management":           false,
		"remote_support":            false,
	}
	for _, tool := range catalog.Tools {
		if tool.Title == "" || tool.OfficialURL == "" {
			t.Fatalf("tool %q missing title or official_url", tool.ID)
		}
		if len(tool.Architectures) == 0 || len(tool.Environment) == 0 {
			t.Fatalf("tool %q missing architectures or environment", tool.ID)
		}
		if _, ok := requiredCategories[tool.Category]; ok {
			requiredCategories[tool.Category] = true
		}
		if tool.IntegrationStatus == "policy_blocked" {
			if tool.Risk != "policy_blocked" || tool.DownloadMode != "none" {
				t.Fatalf("policy_blocked tool %q has risk=%q download_mode=%q", tool.ID, tool.Risk, tool.DownloadMode)
			}
		}
		switch tool.DownloadMode {
		case "build_time_fetch", "technician_cache", "manual_official":
			if !tool.ChecksumRequired {
				t.Fatalf("tool %q download_mode=%q requires checksum_required=true", tool.ID, tool.DownloadMode)
			}
		case "bundled_first_party", "winpe_builtin", "none":
			if tool.ChecksumRequired {
				t.Fatalf("tool %q download_mode=%q expects checksum_required=false", tool.ID, tool.DownloadMode)
			}
		}
	}
	for category, seen := range requiredCategories {
		if !seen {
			t.Fatalf("catalog missing category %q", category)
		}
	}
}

func TestToolCatalogSchemaRejectsInvalidInstances(t *testing.T) {
	t.Parallel()
	schema := compileCatalogSchema(t)

	validTool := map[string]any{
		"id":                  "example-tool",
		"title":               "Example",
		"category":            "network",
		"license":             "mit",
		"commercial_use":      "allowed",
		"redistribution":      "allowed",
		"official_url":        "https://example.invalid/tool",
		"download_mode":       "technician_cache",
		"checksum_required":   true,
		"architectures":       []any{"amd64"},
		"environment":         []any{"winpe"},
		"cli_or_gui":          "cli",
		"risk":                "low",
		"size":                "small",
		"dependencies":        []any{},
		"integration_status":  "candidate",
	}

	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantErr bool
	}{
		{
			name:    "valid minimal catalog",
			mutate:  func(map[string]any) {},
			wantErr: false,
		},
		{
			name: "fetch without checksum",
			mutate: func(doc map[string]any) {
				tools := doc["tools"].([]any)
				tool := cloneMap(tools[0].(map[string]any))
				tool["checksum_required"] = false
				doc["tools"] = []any{tool}
			},
			wantErr: true,
		},
		{
			name: "policy_blocked with download",
			mutate: func(doc map[string]any) {
				tools := doc["tools"].([]any)
				tool := cloneMap(tools[0].(map[string]any))
				tool["integration_status"] = "policy_blocked"
				tool["risk"] = "policy_blocked"
				tool["download_mode"] = "manual_official"
				tool["checksum_required"] = true
				tool["redistribution"] = "prohibited"
				doc["tools"] = []any{tool}
			},
			wantErr: true,
		},
		{
			name: "unknown category",
			mutate: func(doc map[string]any) {
				tools := doc["tools"].([]any)
				tool := cloneMap(tools[0].(map[string]any))
				tool["category"] = "strelec_packs"
				doc["tools"] = []any{tool}
			},
			wantErr: true,
		},
		{
			name: "missing required profile field",
			mutate: func(doc map[string]any) {
				profiles := doc["profiles"].([]any)
				profile := cloneMap(profiles[0].(map[string]any))
				delete(profile, "tool_ids")
				doc["profiles"] = []any{profile}
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			doc := map[string]any{
				"schema_version": 1,
				"profiles": []any{
					map[string]any{
						"id":          "minimal-diagnostics",
						"title":       "Minimal",
						"description": "Test profile",
						"tool_ids":    []any{"example-tool"},
					},
				},
				"tools": []any{cloneMap(validTool)},
			}
			test.mutate(doc)
			raw, err := json.Marshal(doc)
			if err != nil {
				t.Fatalf("Marshal() error = %v", err)
			}
			var instance any
			if err := json.Unmarshal(raw, &instance); err != nil {
				t.Fatalf("Unmarshal instance: %v", err)
			}
			err = schema.Validate(instance)
			if test.wantErr && err == nil {
				t.Fatal("schema.Validate() error = nil, want failure")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("schema.Validate() error = %v", err)
			}
		})
	}
}

func TestToolCatalogHasNoDirectBinaryDownloadURLs(t *testing.T) {
	t.Parallel()
	catalog := loadCatalog(t)
	forbiddenSuffixes := []string{".exe", ".msi", ".zip", ".7z", ".rar", ".iso", ".wim", ".cab"}
	for _, tool := range catalog.Tools {
		url := tool.OfficialURL
		for _, suffix := range forbiddenSuffixes {
			if len(url) >= len(suffix) && equalFoldSuffix(url, suffix) {
				t.Fatalf("tool %q official_url looks like a direct binary download: %s", tool.ID, url)
			}
		}
	}
}

type catalogFile struct {
	SchemaVersion int `json:"schema_version"`
	Profiles      []struct {
		ID      string   `json:"id"`
		ToolIDs []string `json:"tool_ids"`
	} `json:"profiles"`
	Tools []struct {
		ID                 string   `json:"id"`
		Title              string   `json:"title"`
		Category           string   `json:"category"`
		OfficialURL        string   `json:"official_url"`
		DownloadMode       string   `json:"download_mode"`
		ChecksumRequired   bool     `json:"checksum_required"`
		Architectures      []string `json:"architectures"`
		Environment        []string `json:"environment"`
		Risk               string   `json:"risk"`
		IntegrationStatus  string   `json:"integration_status"`
	} `json:"tools"`
}

func loadCatalog(t *testing.T) catalogFile {
	t.Helper()
	raw := readRepoFile(t, "manifests", "tool-catalog.json")
	var catalog catalogFile
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatalf("Unmarshal catalog: %v", err)
	}
	return catalog
}

func compileCatalogSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	raw := readRepoFile(t, "manifests", "tool-catalog.schema.json")
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	if err := compiler.AddResource(schemaURL, bytes.NewReader(raw)); err != nil {
		t.Fatalf("AddResource() error = %v", err)
	}
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return schema
}

func readRepoFile(t *testing.T, parts ...string) []byte {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	pathParts := append([]string{filepath.Dir(file), "..", ".."}, parts...)
	path := filepath.Clean(filepath.Join(pathParts...))
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return raw
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func equalFoldSuffix(value, suffix string) bool {
	if len(value) < len(suffix) {
		return false
	}
	a := value[len(value)-len(suffix):]
	if len(a) != len(suffix) {
		return false
	}
	for i := 0; i < len(suffix); i++ {
		ac := a[i]
		bc := suffix[i]
		if ac >= 'A' && ac <= 'Z' {
			ac += 'a' - 'A'
		}
		if bc >= 'A' && bc <= 'Z' {
			bc += 'a' - 'A'
		}
		if ac != bc {
			return false
		}
	}
	return true
}
