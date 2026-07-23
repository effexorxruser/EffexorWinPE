package agentloop_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

func TestAgentResultSchemaLoads(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", "contracts"))
	if err != nil {
		t.Fatalf("abs: %v", err)
	}
	schemaPath := filepath.Join(root, "agent-result.schema.json")
	if _, err := os.Stat(schemaPath); err != nil {
		t.Fatalf("stat schema: %v", err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	// diagnosis.schema.json is referenced; load from the contracts directory.
	if err := compiler.AddResource("https://effexorwinpe.local/contracts/agent-result.schema.json", mustOpen(t, schemaPath)); err != nil {
		t.Fatalf("AddResource agent-result: %v", err)
	}
	diagnosisPath := filepath.Join(root, "diagnosis.schema.json")
	if err := compiler.AddResource("https://effexorwinpe.local/contracts/diagnosis.schema.json", mustOpen(t, diagnosisPath)); err != nil {
		t.Fatalf("AddResource diagnosis: %v", err)
	}
	if _, err := compiler.Compile("https://effexorwinpe.local/contracts/agent-result.schema.json"); err != nil {
		t.Fatalf("Compile() error = %v", err)
	}
}

func mustOpen(t *testing.T, path string) *os.File {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	t.Cleanup(func() { _ = file.Close() })
	return file
}
