// Package definition holds JSON tool schemas loaded from definitions/*.json.
package definition

import (
	"encoding/json"
)

// Definition describes a tool name, human description, and JSON Schema parameters.
// See definitions/create_location.json for an example document.
type Definition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
