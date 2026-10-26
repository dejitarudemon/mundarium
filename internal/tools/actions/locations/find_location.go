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
	_ tools.Tool = (*FindLocationsTool)(nil)
)

// FindLocationsTool searches locations via the locations service.
type FindLocationsTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindLocationsTool loads the tool definition from file and binds service.
// logger may be nil.
func NewFindLocationsTool(file string, service *servicelocation.Service, logger logger.Logger) (*FindLocationsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &FindLocationsTool{service: service, definition: def, logger: logger}, nil
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
func (t *FindLocationsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_locations", "type", "FindLocationsTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindLocations(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_locations", "type", "FindLocationsTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute find_locations", "type", "FindLocationsTool", "func", "Execute", "error", err)
		return nil, err
	}

	debug(t.logger, "find_locations is executed", "type", "FindLocationsTool", "func", "Execute", "scope_id", in.ScopeID)
	return raw, nil
}

func (t *FindLocationsTool) unmarshal(args json.RawMessage) (servicelocation.FindLocationsInput, error) {
	result := servicelocation.FindLocationsInput{}
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.FindLocationsInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *FindLocationsTool) marshal(output servicelocation.FindLocationsOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
