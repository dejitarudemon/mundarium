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
	_ tools.Tool = (*CreateConnectionTool)(nil)
)

// CreateConnectionTool creates a location connection via the locations service.
type CreateConnectionTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateConnectionTool loads the tool definition from file and binds service.
// logger may be nil.
func NewCreateConnectionTool(file string, service *servicelocation.Service, logger logger.Logger) (*CreateConnectionTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateConnectionTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *CreateConnectionTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that create_connection is not safe to retry without journaling.
func (t *CreateConnectionTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals create-connection args, calls the service, and returns JSON output.
func (t *CreateConnectionTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_connection", "type", "CreateConnectionTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateConnection(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_connection", "type", "CreateConnectionTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute create_connection", "type", "CreateConnectionTool", "func", "Execute", "error", err)
		return nil, err
	}

	info(t.logger, "create_connection is executed", "type", "CreateConnectionTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateConnectionTool) unmarshal(args json.RawMessage) (servicelocation.CreateConnectionInput, error) {
	var result servicelocation.CreateConnectionInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.CreateConnectionInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *CreateConnectionTool) marshal(output servicelocation.CreateConnectionOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
