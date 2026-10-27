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
	_ tools.Tool = (*UpdateConnectionBidirectionalTool)(nil)
)

// UpdateConnectionBidirectionalTool sets a connection's bidirectional flag via the locations service.
type UpdateConnectionBidirectionalTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionBidirectionalArgs is the JSON payload accepted by update_connection_bidirectional.
type UpdateConnectionBidirectionalArgs struct {
	ID            repositories.ID `json:"id"`
	Bidirectional bool            `json:"bidirectional"`
}

// UpdateConnectionBidirectionalResult is the JSON payload returned by update_connection_bidirectional.
type UpdateConnectionBidirectionalResult struct {
	ID            repositories.ID `json:"id"`
	Bidirectional bool            `json:"bidirectional"`
}

// NewUpdateConnectionBidirectionalTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionBidirectionalTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionBidirectionalTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionBidirectionalTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionBidirectionalTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_bidirectional is safe to retry without journaling.
func (t *UpdateConnectionBidirectionalTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, patches bidirectional, and returns JSON output.
func (t *UpdateConnectionBidirectionalTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_bidirectional", "type", "UpdateConnectionBidirectionalTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionBidirectional(ctx, in.ID, in.Bidirectional); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_bidirectional", "type", "UpdateConnectionBidirectionalTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionBidirectionalResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_bidirectional", "type", "UpdateConnectionBidirectionalTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_bidirectional is executed", "type", "UpdateConnectionBidirectionalTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionBidirectionalTool) unmarshal(args json.RawMessage) (UpdateConnectionBidirectionalArgs, error) {
	var result UpdateConnectionBidirectionalArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionBidirectionalArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionBidirectionalArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionBidirectionalTool) marshal(output UpdateConnectionBidirectionalResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
