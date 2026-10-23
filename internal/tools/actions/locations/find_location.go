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
	_ tools.Tool = (*FindLocationsTool)(nil)
)

// FindLocationsTool searches locations via the locations service.
type FindLocationsTool struct {
	service    *servicelocation.Service
	definition definition.Definition
}

// NewFindLocationsTool loads the tool definition from file and binds service.
func NewFindLocationsTool(file string, service *servicelocation.Service) (*FindLocationsTool, error) {
	if service == nil {
		return nil, actions.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &FindLocationsTool{service: service, definition: def}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *FindLocationsTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that find_locations is safe to retry without journaling.
func (t *FindLocationsTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals find-locations args, calls the service, and returns JSON output.
func (t *FindLocationsTool) Execute(ctx execution.ExecutionContext, args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		return nil, err
	}

	result, err := t.service.FindLocations(ctx, in)
	if err != nil {
		return nil, err
	}

	return t.marshal(result)
}

func (t *FindLocationsTool) unmarshal(args json.RawMessage) (inputslocation.FindLocationsInput, error) {
	result := inputslocation.FindLocationsInput{}
	if err := json.Unmarshal(args, &result); err != nil {
		return inputslocation.FindLocationsInput{}, err
	}

	return result, nil
}

func (t *FindLocationsTool) marshal(output outputslocation.FindLocationsOutput) (json.RawMessage, error) {
	return json.Marshal(output)
}
