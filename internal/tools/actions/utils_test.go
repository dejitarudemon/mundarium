package actions_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/tools/actions"
)

func definitionsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/tools/actions -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "definitions"))
}

func TestLoadDefinition(t *testing.T) {
	path := filepath.Join(definitionsDir(t), "create_location.json")

	def, err := actions.LoadDefinition(path)
	if err != nil {
		t.Fatalf("LoadDefinition: %v", err)
	}
	if def.Name != "create_location" {
		t.Fatalf("want name create_location, got %q", def.Name)
	}
	if def.Description == "" {
		t.Fatal("expected non-empty description")
	}
	if len(def.Parameters) == 0 {
		t.Fatal("expected parameters payload")
	}

	var params map[string]any
	if err := json.Unmarshal(def.Parameters, &params); err != nil {
		t.Fatalf("parameters json: %v", err)
	}
	if params["type"] != "object" {
		t.Fatalf("want parameters.type=object, got %#v", params["type"])
	}
}

func TestLoadDefinition_MissingFile(t *testing.T) {
	_, err := actions.LoadDefinition(filepath.Join(t.TempDir(), "missing.json"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadDefinition_InvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	_, err := actions.LoadDefinition(path)
	if err == nil {
		t.Fatal("expected error for invalid json")
	}
}
