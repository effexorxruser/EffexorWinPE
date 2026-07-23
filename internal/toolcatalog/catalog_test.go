package toolcatalog_test

import (
	"bytes"
	"encoding/json"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
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

func TestToolCatalogIntegrity(t *testing.T) {
	t.Parallel()
	catalog := loadCatalog(t)

	toolIDs := make(map[string]struct{}, len(catalog.Tools))
	toolsByID := make(map[string]catalogTool, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		if _, exists := toolIDs[tool.ID]; exists {
			t.Fatalf("duplicate tool id %q", tool.ID)
		}
		toolIDs[tool.ID] = struct{}{}
		toolsByID[tool.ID] = tool
	}

	profileIDs := make(map[string]struct{}, len(catalog.Profiles))
	requiredProfiles := map[string]struct{}{
		"minimal-diagnostics": {},
		"technician-standard": {},
		"data-recovery":       {},
		"network-enabled":     {},
		"multiboot-extras":    {},
	}
	for _, profile := range catalog.Profiles {
		if _, exists := profileIDs[profile.ID]; exists {
			t.Fatalf("duplicate profile id %q", profile.ID)
		}
		profileIDs[profile.ID] = struct{}{}
		delete(requiredProfiles, profile.ID)

		for _, toolID := range profile.ToolIDs {
			if _, ok := toolIDs[toolID]; !ok {
				t.Fatalf("profile %q references unknown tool id %q", profile.ID, toolID)
			}
		}
		assertProfileMaturity(t, profile, toolsByID)
	}
	for missing := range requiredProfiles {
		t.Fatalf("missing required release profile %q", missing)
	}

	deps := make(map[string][]string, len(catalog.Tools))
	for _, tool := range catalog.Tools {
		for _, dep := range tool.Dependencies {
			if dep == tool.ID {
				t.Fatalf("tool %q has a self-dependency", tool.ID)
			}
			if _, ok := toolIDs[dep]; !ok {
				t.Fatalf("tool %q depends on unknown id %q", tool.ID, dep)
			}
		}
		deps[tool.ID] = append([]string(nil), tool.Dependencies...)
	}
	if cycle := findDependencyCycle(deps); cycle != "" {
		t.Fatalf("dependency cycle detected: %s", cycle)
	}

	minimal := findProfile(t, catalog, "minimal-diagnostics")
	for _, toolID := range minimal.ToolIDs {
		tool := toolsByID[toolID]
		switch tool.Risk {
		case "read_only", "low":
		default:
			t.Fatalf("minimal-diagnostics tool %q has risk %q; minimal profile must stay read-only", toolID, tool.Risk)
		}
		if toolID == "winpe-diskpart" {
			t.Fatal("minimal-diagnostics must not include winpe-diskpart")
		}
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
		if tool.License == "proprietary-effexor" {
			t.Fatalf("tool %q still uses proprietary-effexor; first-party code is MIT", tool.ID)
		}
		if strings.HasPrefix(tool.ID, "effexor") && tool.License != "mit" {
			t.Fatalf("first-party tool %q license = %q, want mit", tool.ID, tool.License)
		}
		if _, ok := requiredCategories[tool.Category]; ok {
			requiredCategories[tool.Category] = true
		}
		if tool.IntegrationStatus == "policy_blocked" {
			if tool.Risk != "policy_blocked" || tool.DownloadMode != "none" || tool.LicenseReviewStatus != "blocked" {
				t.Fatalf("policy_blocked tool %q has risk=%q download_mode=%q license_review_status=%q",
					tool.ID, tool.Risk, tool.DownloadMode, tool.LicenseReviewStatus)
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
		assertLicenseReviewConsistency(t, tool)
	}
	for category, seen := range requiredCategories {
		if !seen {
			t.Fatalf("catalog missing category %q", category)
		}
	}
	if _, ok := findToolOptional(catalog, "oem-firmware-updater"); ok {
		t.Fatal("generic oem-firmware-updater entry must be removed")
	}
	if _, ok := findToolOptional(catalog, "oem-firmware-updater-policy"); !ok {
		t.Fatal("expected oem-firmware-updater-policy template")
	}
}

func TestToolCatalogOfficialURLs(t *testing.T) {
	t.Parallel()
	catalog := loadCatalog(t)
	forbiddenExt := map[string]struct{}{
		".exe": {}, ".msi": {}, ".zip": {}, ".7z": {}, ".rar": {},
		".iso": {}, ".wim": {}, ".cab": {}, ".dmg": {}, ".pkg": {},
	}
	for _, tool := range catalog.Tools {
		assertHTTPSDocumentURL(t, "official_url", tool.ID, tool.OfficialURL, forbiddenExt)
		assertHTTPSDocumentURL(t, "review_source", tool.ID, tool.ReviewSource, forbiddenExt)
	}
}

func TestToolCatalogSchemaRejectsInvalidInstances(t *testing.T) {
	t.Parallel()
	schema := compileCatalogSchema(t)

	validTool := map[string]any{
		"id":                    "example-tool",
		"title":                 "Example",
		"category":              "network",
		"license":               "mit",
		"commercial_use":        "allowed",
		"redistribution":        "allowed",
		"official_url":          "https://example.invalid/tool",
		"download_mode":         "technician_cache",
		"checksum_required":     true,
		"architectures":         []any{"amd64"},
		"environment":           []any{"winpe"},
		"cli_or_gui":            "cli",
		"risk":                  "low",
		"size":                  "small",
		"dependencies":          []any{},
		"integration_status":    "candidate",
		"license_review_status": "reviewed",
		"review_source":         "https://example.invalid/license",
		"reviewed_at":           "2026-07-23",
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
				tool["license_review_status"] = "blocked"
				tool["reviewed_at"] = nil
				doc["tools"] = []any{tool}
			},
			wantErr: true,
		},
		{
			name: "http official_url",
			mutate: func(doc map[string]any) {
				tools := doc["tools"].([]any)
				tool := cloneMap(tools[0].(map[string]any))
				tool["official_url"] = "http://example.invalid/tool"
				doc["tools"] = []any{tool}
			},
			wantErr: true,
		},
		{
			name: "pending with final commercial_use",
			mutate: func(doc map[string]any) {
				tools := doc["tools"].([]any)
				tool := cloneMap(tools[0].(map[string]any))
				tool["license_review_status"] = "pending"
				tool["commercial_use"] = "allowed"
				tool["redistribution"] = "review_required"
				tool["reviewed_at"] = nil
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
			name: "missing profile maturity",
			mutate: func(doc map[string]any) {
				profiles := doc["profiles"].([]any)
				profile := cloneMap(profiles[0].(map[string]any))
				delete(profile, "maturity")
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
						"maturity":    "conceptual",
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

type catalogFile struct {
	SchemaVersion int              `json:"schema_version"`
	Profiles      []catalogProfile `json:"profiles"`
	Tools         []catalogTool    `json:"tools"`
}

type catalogProfile struct {
	ID       string   `json:"id"`
	Maturity string   `json:"maturity"`
	ToolIDs  []string `json:"tool_ids"`
}

type catalogTool struct {
	ID                  string   `json:"id"`
	Title               string   `json:"title"`
	Category            string   `json:"category"`
	License             string   `json:"license"`
	CommercialUse       string   `json:"commercial_use"`
	Redistribution      string   `json:"redistribution"`
	OfficialURL         string   `json:"official_url"`
	DownloadMode        string   `json:"download_mode"`
	ChecksumRequired    bool     `json:"checksum_required"`
	Architectures       []string `json:"architectures"`
	Environment         []string `json:"environment"`
	Risk                string   `json:"risk"`
	Dependencies        []string `json:"dependencies"`
	IntegrationStatus   string   `json:"integration_status"`
	LicenseReviewStatus string   `json:"license_review_status"`
	ReviewSource        string   `json:"review_source"`
	ReviewedAt          *string  `json:"reviewed_at"`
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
	compiler.AssertFormat = true
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

func findProfile(t *testing.T, catalog catalogFile, id string) catalogProfile {
	t.Helper()
	for _, profile := range catalog.Profiles {
		if profile.ID == id {
			return profile
		}
	}
	t.Fatalf("profile %q not found", id)
	return catalogProfile{}
}

func findToolOptional(catalog catalogFile, id string) (catalogTool, bool) {
	for _, tool := range catalog.Tools {
		if tool.ID == id {
			return tool, true
		}
	}
	return catalogTool{}, false
}

func assertProfileMaturity(t *testing.T, profile catalogProfile, toolsByID map[string]catalogTool) {
	t.Helper()
	hasIncomplete := false
	for _, toolID := range profile.ToolIDs {
		switch toolsByID[toolID].IntegrationStatus {
		case "candidate", "planned":
			hasIncomplete = true
		}
	}
	switch profile.Maturity {
	case "conceptual", "experimental":
		return
	case "release_candidate", "released":
		if hasIncomplete {
			t.Fatalf("profile %q maturity %q includes candidate/planned tools", profile.ID, profile.Maturity)
		}
	default:
		t.Fatalf("profile %q has unknown maturity %q", profile.ID, profile.Maturity)
	}
}

func assertLicenseReviewConsistency(t *testing.T, tool catalogTool) {
	t.Helper()
	switch tool.LicenseReviewStatus {
	case "pending":
		if tool.CommercialUse != "unknown" || tool.Redistribution != "review_required" {
			t.Fatalf("pending tool %q must use commercial_use=unknown redistribution=review_required", tool.ID)
		}
		if tool.ReviewedAt != nil {
			t.Fatalf("pending tool %q reviewed_at must be null", tool.ID)
		}
	case "reviewed":
		if tool.ReviewedAt == nil || *tool.ReviewedAt == "" {
			t.Fatalf("reviewed tool %q missing reviewed_at", tool.ID)
		}
	case "not_required", "blocked":
		if tool.ReviewedAt != nil {
			t.Fatalf("%s tool %q reviewed_at must be null", tool.LicenseReviewStatus, tool.ID)
		}
	default:
		t.Fatalf("tool %q has unknown license_review_status %q", tool.ID, tool.LicenseReviewStatus)
	}
}

func assertHTTPSDocumentURL(t *testing.T, field, toolID, raw string, forbiddenExt map[string]struct{}) {
	t.Helper()
	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("tool %q %s parse error: %v", toolID, field, err)
	}
	if parsed.Scheme != "https" {
		t.Fatalf("tool %q %s scheme = %q, want https", toolID, field, parsed.Scheme)
	}
	if parsed.Host == "" {
		t.Fatalf("tool %q %s missing host: %s", toolID, field, raw)
	}
	ext := strings.ToLower(path.Ext(parsed.Path))
	if _, blocked := forbiddenExt[ext]; blocked {
		t.Fatalf("tool %q %s looks like a direct binary download (%s): %s", toolID, field, ext, raw)
	}
}

func findDependencyCycle(deps map[string][]string) string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(deps))
	var stack []string
	var dfs func(string) string
	dfs = func(node string) string {
		color[node] = gray
		stack = append(stack, node)
		for _, next := range deps[node] {
			switch color[next] {
			case white:
				if cycle := dfs(next); cycle != "" {
					return cycle
				}
			case gray:
				start := 0
				for i, id := range stack {
					if id == next {
						start = i
						break
					}
				}
				cycle := append(append([]string{}, stack[start:]...), next)
				return strings.Join(cycle, " -> ")
			}
		}
		stack = stack[:len(stack)-1]
		color[node] = black
		return ""
	}
	for id := range deps {
		if color[id] == white {
			if cycle := dfs(id); cycle != "" {
				return cycle
			}
		}
	}
	return ""
}
