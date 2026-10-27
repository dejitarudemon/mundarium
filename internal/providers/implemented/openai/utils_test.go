package openai

import (
	"encoding/json"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

func TestEndpoint(t *testing.T) {
	got := endpoint("http://localhost:8080", "/", "v1", "models")
	if got != "http://localhost:8080/v1/models" {
		t.Fatalf("endpoint: got %q", got)
	}
}

func TestConvertTools(t *testing.T) {
	params := json.RawMessage(`{"type":"object","properties":{}}`)
	got, err := convertTools([]definition.Definition{{
		Name:        "create_location",
		Description: "create a location",
		Parameters:  params,
	}})
	if err != nil {
		t.Fatalf("convertTools: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 tool, got %d", len(got))
	}
}

func TestConvertTools_InvalidParameters(t *testing.T) {
	_, err := convertTools([]definition.Definition{{
		Name:       "bad",
		Parameters: json.RawMessage(`{`),
	}})
	if err == nil {
		t.Fatal("want invalid parameters error")
	}
}
