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
	_ tools.Tool = (*UpdateConnectionExpiresAtTool)(nil)
)

// UpdateConnectionExpiresAtTool sets or clears a connection's expiry via the locations service.
type UpdateConnectionExpiresAtTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// UpdateConnectionExpiresAtArgs is the JSON payload accepted by update_connection_expires_at.
type UpdateConnectionExpiresAtArgs struct {
	ID        repositories.ID `json:"id"`
	ExpiresAt *int64          `json:"expires_at"`
}

// UpdateConnectionExpiresAtResult is the JSON payload returned by update_connection_expires_at.
type UpdateConnectionExpiresAtResult struct {
	ID        repositories.ID `json:"id"`
	ExpiresAt *int64          `json:"expires_at"`
}

// NewUpdateConnectionExpiresAtTool loads the tool definition from file and binds service.
// logger may be nil.
func NewUpdateConnectionExpiresAtTool(file string, service *servicelocation.Service, logger logger.Logger) (*UpdateConnectionExpiresAtTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &UpdateConnectionExpiresAtTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *UpdateConnectionExpiresAtTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that update_connection_expires_at is safe to retry without journaling.
func (t *UpdateConnectionExpiresAtTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals args, patches expiry, and returns JSON output.
func (t *UpdateConnectionExpiresAtTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_expires_at", "type", "UpdateConnectionExpiresAtTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.PatchConnectionExpiresAt(ctx, in.ID, in.ExpiresAt); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute update_connection_expires_at", "type", "UpdateConnectionExpiresAtTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(UpdateConnectionExpiresAtResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute update_connection_expires_at", "type", "UpdateConnectionExpiresAtTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "update_connection_expires_at is executed", "type", "UpdateConnectionExpiresAtTool", "func", "Execute", "id", in.ID)
	return raw, nil
}

func (t *UpdateConnectionExpiresAtTool) unmarshal(args json.RawMessage) (UpdateConnectionExpiresAtArgs, error) {
	var result UpdateConnectionExpiresAtArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return UpdateConnectionExpiresAtArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return UpdateConnectionExpiresAtArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrConnectionIDZero)
	}

	return result, nil
}

func (t *UpdateConnectionExpiresAtTool) marshal(output UpdateConnectionExpiresAtResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
