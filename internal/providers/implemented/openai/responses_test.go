package openai_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
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

func TestNewSuccessfulToolResponse(t *testing.T) {
	msg := openai.NewSuccessfulToolResponse("call-1", json.RawMessage(`{"id":1}`))
	if msg.OfTool == nil {
		t.Fatal("want tool message")
	}
	if msg.OfTool.ToolCallID != "call-1" {
		t.Fatalf("tool_call_id: want call-1, got %q", msg.OfTool.ToolCallID)
	}
	if got := msg.OfTool.Content.OfString.Or(""); got != `{"id":1}` {
		t.Fatalf("content: want answer json, got %q", got)
	}

	empty := openai.NewSuccessfulToolResponse("call-2", nil)
	if got := empty.OfTool.Content.OfString.Or(""); got != "null" {
		t.Fatalf("nil answer: want null, got %q", got)
	}
}

func TestNewFailToolResponse(t *testing.T) {
	msg := openai.NewFailToolResponse("call-1", errors.New("boom"))
	if msg.OfTool == nil {
		t.Fatal("want tool message")
	}
	if msg.OfTool.ToolCallID != "call-1" {
		t.Fatalf("tool_call_id: want call-1, got %q", msg.OfTool.ToolCallID)
	}
	content := msg.OfTool.Content.OfString.Or("")
	if !strings.Contains(content, `"error"`) || !strings.Contains(content, "boom") {
		t.Fatalf("content missing error payload: %q", content)
	}
	if !strings.Contains(content, `"is_retryable":false`) {
		t.Fatalf("plain error must not be retryable: %q", content)
	}

	retryable := openai.NewFailToolResponse("call-2", toolerrs.NewInternalError(errors.New("db")))
	retryContent := retryable.OfTool.Content.OfString.Or("")
	if !strings.Contains(retryContent, `"is_retryable":true`) {
		t.Fatalf("InternalError must be retryable: %q", retryContent)
	}
}
