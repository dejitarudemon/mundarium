// Package locations implements location-related tools.
package locations

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	servicelocation "github.com/dejitarudemon/mundarium/internal/services/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*CreateLocationTool)(nil)
)

// CreateLocationTool creates a location via the locations service.
type CreateLocationTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateLocationTool loads the tool definition from file and binds service.
// logger may be nil.
func NewCreateLocationTool(file string, service *servicelocation.Service, logger logger.Logger) (*CreateLocationTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateLocationTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *CreateLocationTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that create_location is not safe to retry without journaling.
func (t *CreateLocationTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals create-location args, calls the service, and returns JSON output.
func (t *CreateLocationTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	location, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_location", "type", "CreateLocationTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateLocation(ctx, location)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_location", "type", "CreateLocationTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute create_location", "type", "CreateLocationTool", "func", "Execute", "error", err)
		return nil, err
	}

	info(t.logger, "create_location is executed", "type", "CreateLocationTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateLocationTool) unmarshal(args json.RawMessage) (servicelocation.CreateLocationInput, error) {
	result := servicelocation.CreateLocationInput{}
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.CreateLocationInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *CreateLocationTool) marshal(output servicelocation.CreateLocationOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
