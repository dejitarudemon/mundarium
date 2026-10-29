package locations

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	servicelocation "github.com/dejitarudemon/mundarium/internal/services/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*DeleteConnectionTool)(nil)
)

// DeleteConnectionTool deletes a location connection via the locations service.
type DeleteConnectionTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// DeleteConnectionArgs is the JSON payload accepted by delete_connection.
type DeleteConnectionArgs struct {
	ID repositories.ID `json:"id"`
}

// DeleteConnectionResult is the JSON payload returned by delete_connection.
type DeleteConnectionResult struct {
	ID repositories.ID `json:"id"`
}

// NewDeleteConnectionTool loads the tool definition from file and binds service.
// logger may be nil.
func NewDeleteConnectionTool(file string, service *servicelocation.Service, logger logger.Logger) (*DeleteConnectionTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &DeleteConnectionTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *DeleteConnectionTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that delete_connection is not safe to retry without journaling.
func (t *DeleteConnectionTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals args, deletes the connection, and returns JSON output.
func (t *DeleteConnectionTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute delete_connection", "type", "DeleteConnectionTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.DeleteConnection(ctx, in.ID); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute delete_connection", "type", "DeleteConnectionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(DeleteConnectionResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute delete_connection", "type", "DeleteConnectionTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "delete_connection is executed", "type", "DeleteConnectionTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *DeleteConnectionTool) unmarshal(args json.RawMessage) (DeleteConnectionArgs, error) {
	var result DeleteConnectionArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return DeleteConnectionArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return DeleteConnectionArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *DeleteConnectionTool) marshal(output DeleteConnectionResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
