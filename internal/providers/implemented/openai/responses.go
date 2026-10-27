package openai

import (
	"encoding/json"
	"errors"
	"slices"

	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/openai/openai-go/v3"
)

// Model is one entry from an OpenAI-compatible /v1/models response.
type Model struct {
	ID     string `json:"id"`
	Object string `json:"object"`
}

// ModelsResponse is the list payload returned by /v1/models.
type ModelsResponse struct {
	Data []Model `json:"data"`
}

// HasModel reports whether id is present as a model object.
func (m ModelsResponse) HasModel(id string) bool {
	return slices.ContainsFunc(m.Data, func(m Model) bool {
		return m.Object == "model" && m.ID == id
	})
}

// NewModelsResponse unmarshals a /v1/models JSON body.
func NewModelsResponse(raw json.RawMessage) (ModelsResponse, error) {
	var result ModelsResponse

	if err := json.Unmarshal(raw, &result); err != nil {
		return ModelsResponse{}, err
	}

	return result, nil
}

// toolFailPayload is the JSON body sent back to the model when a tool call fails.
type toolFailPayload struct {
	Error       string `json:"error"`
	IsRetryable bool   `json:"is_retryable"`
}

// NewSuccessfulToolResponse builds a tool-role message with the executor result body.
func NewSuccessfulToolResponse(toolCallID string, answer json.RawMessage) openai.ChatCompletionMessageParamUnion {
	content := "null"
	if len(answer) > 0 {
		content = string(answer)
	}
	return openai.ToolMessage(content, toolCallID)
}

// NewFailToolResponse builds a tool-role message describing an executor failure.
// IsRetryable is taken from [toolerrs.ToolErr] when present; otherwise false.
func NewFailToolResponse(toolCallID string, err error) openai.ChatCompletionMessageParamUnion {
	msg := "unknown error"
	retryable := false
	if err != nil {
		msg = err.Error()
		if te, ok := errors.AsType[toolerrs.ToolErr](err); ok {
			retryable = te.IsRetryable()
		}
	}
	payload, marshalErr := json.Marshal(toolFailPayload{
		Error:       msg,
		IsRetryable: retryable,
	})
	if marshalErr != nil {
		payload = []byte(`{"error":"tool execution failed","is_retryable":false}`)
	}
	return openai.ToolMessage(string(payload), toolCallID)
}
