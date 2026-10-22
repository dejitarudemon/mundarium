// Package actions provides shared helpers for concrete tool implementations.
package actions

import (
	"encoding/json"
	"os"

	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

// LoadDefinition reads and unmarshals a tool definition JSON file.
func LoadDefinition(file string) (definition.Definition, error) {
	content, err := os.ReadFile(file)
	if err != nil {
		return definition.Definition{}, err
	}

	def := definition.Definition{}
	if err := json.Unmarshal(content, &def); err != nil {
		return definition.Definition{}, err
	}

	return def, nil
}
