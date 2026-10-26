package openai_test

import (
	"encoding/json"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
)

func TestModelsResponse_HasModel(t *testing.T) {
	raw := json.RawMessage(`{"data":[{"id":"gpt-test","object":"model"},{"id":"other","object":"file"}]}`)
	got, err := openai.NewModelsResponse(raw)
	if err != nil {
		t.Fatalf("NewModelsResponse: %v", err)
	}
	if !got.HasModel("gpt-test") {
		t.Fatal("want gpt-test present")
	}
	if got.HasModel("missing") || got.HasModel("other") {
		t.Fatal("only object=model entries should match")
	}
}

func TestNewModelsResponse_InvalidJSON(t *testing.T) {
	if _, err := openai.NewModelsResponse(json.RawMessage(`{`)); err == nil {
		t.Fatal("want unmarshal error")
	}
}
