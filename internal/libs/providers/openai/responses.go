package openai

import (
	"encoding/json"
	"slices"
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
