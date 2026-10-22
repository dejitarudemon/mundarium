// Package locations implements location-related tools.
package locations

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	inputslocation "github.com/dejitarudemon/mundarium/internal/libs/inputs/locations"
	outputslocation "github.com/dejitarudemon/mundarium/internal/libs/outputs/locations"
	servicelocation "github.com/dejitarudemon/mundarium/internal/services/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

var (
	_ tools.Tool = (*CreateLocationTool)(nil)
)

// CreateLocationTool creates a location via the locations service.
type CreateLocationTool struct {
	service    *servicelocation.Service
	definition definition.Definition
}

// NewCreateLocationTool loads the tool definition from file and binds service.
func NewCreateLocationTool(file string, service *servicelocation.Service) (*CreateLocationTool, error) {
	if service == nil {
		return nil, actions.ServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateLocationTool{service: service, definition: def}, nil
}

// Definition returns the loaded tool schema.
func (t *CreateLocationTool) Definition() definition.Definition {
	return t.definition
}

// Execute unmarshals create-location args, calls the service, and returns JSON output.
func (t *CreateLocationTool) Execute(ctx execution.ExecutionContext, args json.RawMessage) (json.RawMessage, error) {
	location, err := t.unmarshal(args)
	if err != nil {
		return nil, err
	}

	result, err := t.service.CreateLocation(ctx, location)
	if err != nil {
		return nil, err
	}

	return t.marshal(result)
}

func (t *CreateLocationTool) unmarshal(args json.RawMessage) (inputslocation.CreateLocationInput, error) {
	result := inputslocation.CreateLocationInput{}
	if err := json.Unmarshal(args, &result); err != nil {
		return inputslocation.CreateLocationInput{}, err
	}

	return result, nil
}

func (t *CreateLocationTool) marshal(output outputslocation.CreateLocationOutput) (json.RawMessage, error) {
	return json.Marshal(output)
}
