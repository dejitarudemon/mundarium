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
	_ tools.Tool = (*FindConnectionsTool)(nil)
)

// FindConnectionsTool searches location connections via the locations service.
type FindConnectionsTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindConnectionsTool loads the tool definition from file and binds service.
// logger may be nil.
func NewFindConnectionsTool(file string, service *servicelocation.Service, logger logger.Logger) (*FindConnectionsTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &FindConnectionsTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *FindConnectionsTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that find_connections is safe to retry without journaling.
func (t *FindConnectionsTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals find-connections args, calls the service, and returns JSON output.
func (t *FindConnectionsTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_connections", "type", "FindConnectionsTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindConnections(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_connections", "type", "FindConnectionsTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute find_connections", "type", "FindConnectionsTool", "func", "Execute", "error", err)
		return nil, err
	}

	debug(t.logger, "find_connections is executed", "type", "FindConnectionsTool", "func", "Execute", "count", len(result.Connections))
	return raw, nil
}

func (t *FindConnectionsTool) unmarshal(args json.RawMessage) (servicelocation.FindConnectionsInput, error) {
	var result servicelocation.FindConnectionsInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.FindConnectionsInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *FindConnectionsTool) marshal(output servicelocation.FindConnectionsOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
