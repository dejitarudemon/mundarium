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
	_ tools.Tool = (*MoveLocationTool)(nil)
)

// MoveLocationTool reparents a location via the locations service.
type MoveLocationTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// MoveLocationArgs is the JSON payload accepted by move_location.
type MoveLocationArgs struct {
	ID       repositories.ID  `json:"id"`
	ParentID *repositories.ID `json:"parent_id"`
}

// MoveLocationResult is the JSON payload returned by move_location.
type MoveLocationResult struct {
	ID       repositories.ID  `json:"id"`
	ParentID *repositories.ID `json:"parent_id"`
}

// NewMoveLocationTool loads the tool definition from file and binds service.
// logger may be nil.
func NewMoveLocationTool(file string, service *servicelocation.Service, logger logger.Logger) (*MoveLocationTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &MoveLocationTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *MoveLocationTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that move_location is safe to retry without journaling.
func (t *MoveLocationTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals move-location args, calls the service, and returns JSON output.
func (t *MoveLocationTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute move_location", "type", "MoveLocationTool", "func", "Execute", "error", err)
		return nil, err
	}

	if err := t.service.MoveLocation(ctx, in.ID, in.ParentID); err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute move_location", "type", "MoveLocationTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	raw, err := t.marshal(MoveLocationResult(in))
	if err != nil {
		errorf(t.logger, "failed to execute move_location", "type", "MoveLocationTool", "func", "Execute", "id", in.ID, "error", err)
		return nil, err
	}

	info(t.logger, "move_location is executed", "type", "MoveLocationTool", "func", "Execute", "id", in.ID, "parent_id", in.ParentID)
	return raw, nil
}

func (t *MoveLocationTool) unmarshal(args json.RawMessage) (MoveLocationArgs, error) {
	var result MoveLocationArgs
	if err := json.Unmarshal(args, &result); err != nil {
		return MoveLocationArgs{}, toolerrs.NewInvalidRequest(err)
	}
	if result.ID == 0 {
		return MoveLocationArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrLocationIDZero)
	}
	if result.ParentID != nil && *result.ParentID == 0 {
		return MoveLocationArgs{}, toolerrs.NewInvalidRequest(toolerrs.ErrParentIDZero)
	}

	return result, nil
}

func (t *MoveLocationTool) marshal(output MoveLocationResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
