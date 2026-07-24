package agentloop_test

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/effexorxruser/EffexorWinPE/internal/agentloop"
	"github.com/effexorxruser/EffexorWinPE/internal/diagnosis"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestAgentResultSchemaPositiveAndNegativeInstances(t *testing.T) {
	schema := compileAgentResultSchema(t)
	now := time.Date(2026, 7, 23, 12, 0, 0, 0, time.UTC).Format(time.RFC3339)

	assessment := map[string]any{
		"schema_version": diagnosis.SchemaVersion,
		"report_id":      "report-schema-1",
		"generated_at":   now,
		"mode":           diagnosis.ModeOnlineAgent,
		"summary": map[string]any{
			"headline":         "Bounded result",
			"highest_severity": diagnosis.SeverityInfo,
			"finding_count":    1,
		},
		"findings": []any{map[string]any{
			"id":            "finding.one",
			"title":         "One finding",
			"severity":      diagnosis.SeverityInfo,
			"confidence":    diagnosis.ConfidenceLow,
			"rationale":     "Grounded in collected evidence.",
			"evidence_refs": []any{"checks[0].status"},
			"source_refs":   []any{},
		}},
		"questions": []any{},
		"next_steps": []any{map[string]any{
			"id":                    "step-review",
			"title":                 "Review sources",
			"operation":             "review_missing_sources",
			"risk":                  diagnosis.RiskReadOnly,
			"requires_confirmation": false,
			"rationale":             "Stay read-only.",
		}},
		"limitations": []any{"Provisional."},
		"sources":     []any{},
	}

	positives := []map[string]any{
		{
			"schema_version":    agentloop.SchemaVersion,
			"report_id":         "report-schema-1",
			"generated_at":      now,
			"state":             agentloop.StateCompleted,
			"round":             1,
			"assessment":        assessment,
			"evidence_requests": []any{},
			"audit_timeline":    []any{},
			"limitations":       []any{"Provisional."},
		},
		{
			"schema_version": agentloop.SchemaVersion,
			"report_id":      "report-schema-1",
			"generated_at":   now,
			"state":          agentloop.StateNeedsMoreEvidence,
			"round":          1,
			"evidence_requests": []any{map[string]any{
				"id":                   "req-1",
				"operation":            "inspect_network_status",
				"arguments":            map[string]any{},
				"reason":               "Need link state.",
				"expected_information": "Adapter status.",
				"privacy_class":        "network_status",
				"timeout_seconds":      20,
			}},
			"audit_timeline": []any{},
			"limitations":    []any{"Need evidence."},
		},
		{
			"schema_version":    agentloop.SchemaVersion,
			"report_id":         "report-schema-1",
			"generated_at":      now,
			"state":             agentloop.StateBlocked,
			"round":             3,
			"evidence_requests": []any{},
			"block":             map[string]any{"code": "max_rounds_exceeded", "message": "Stopped after 3 rounds."},
			"audit_timeline":    []any{},
			"limitations":       []any{"Blocked."},
		},
		{
			"schema_version":    agentloop.SchemaVersion,
			"report_id":         "report-schema-1",
			"generated_at":      now,
			"state":             agentloop.StateFailed,
			"round":             1,
			"evidence_requests": []any{},
			"failure":           map[string]any{"code": "provider_error", "message": "Provider failed."},
			"audit_timeline":    []any{},
			"limitations":       []any{"Failed closed."},
		},
	}
	for _, instance := range positives {
		if err := schema.Validate(instance); err != nil {
			t.Fatalf("positive Validate(%v) error = %v", instance["state"], err)
		}
	}

	negatives := []map[string]any{
		{ // completed with block
			"schema_version": agentloop.SchemaVersion, "report_id": "report-schema-1", "generated_at": now,
			"state": agentloop.StateCompleted, "round": 1, "assessment": assessment,
			"evidence_requests": []any{}, "block": map[string]any{"code": "x", "message": "no"},
			"audit_timeline": []any{}, "limitations": []any{"x"},
		},
		{ // needs_more_evidence with assessment
			"schema_version": agentloop.SchemaVersion, "report_id": "report-schema-1", "generated_at": now,
			"state": agentloop.StateNeedsMoreEvidence, "round": 1, "assessment": assessment,
			"evidence_requests": []any{map[string]any{
				"id": "req-1", "operation": "inspect_network_status", "arguments": map[string]any{},
				"reason": "Need link state.", "expected_information": "Adapter status.",
				"privacy_class": "network_status", "timeout_seconds": 20,
			}},
			"audit_timeline": []any{}, "limitations": []any{"x"},
		},
		{ // blocked with evidence requests
			"schema_version": agentloop.SchemaVersion, "report_id": "report-schema-1", "generated_at": now,
			"state": agentloop.StateBlocked, "round": 1,
			"evidence_requests": []any{map[string]any{
				"id": "req-1", "operation": "inspect_network_status", "arguments": map[string]any{},
				"reason": "Need link state.", "expected_information": "Adapter status.",
				"privacy_class": "network_status", "timeout_seconds": 20,
			}},
			"block":          map[string]any{"code": "x", "message": "no"},
			"audit_timeline": []any{}, "limitations": []any{"x"},
		},
		{ // failed with assessment
			"schema_version": agentloop.SchemaVersion, "report_id": "report-schema-1", "generated_at": now,
			"state": agentloop.StateFailed, "round": 1, "assessment": assessment,
			"evidence_requests": []any{},
			"failure":           map[string]any{"code": "x", "message": "no"},
			"audit_timeline":    []any{}, "limitations": []any{"x"},
		},
	}
	for index, instance := range negatives {
		if err := schema.Validate(instance); err == nil {
			t.Fatalf("negative case %d Validate() error = nil", index)
		}
	}
}

func compileAgentResultSchema(t *testing.T) *jsonschema.Schema {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "contracts"))
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	for _, name := range []string{"agent-result.schema.json", "diagnosis.schema.json"} {
		raw, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		uri := "https://effexorwinpe.local/contracts/" + name
		if err := compiler.AddResource(uri, bytes.NewReader(raw)); err != nil {
			t.Fatalf("AddResource(%s): %v", name, err)
		}
	}
	schema, err := compiler.Compile("https://effexorwinpe.local/contracts/agent-result.schema.json")
	if err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
	return schema
}
