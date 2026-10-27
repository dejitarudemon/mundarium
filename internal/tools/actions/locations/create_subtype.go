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
	_ tools.Tool = (*CreateSubtypeTool)(nil)
)

// CreateSubtypeTool creates a location subtype via the locations service.
type CreateSubtypeTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewCreateSubtypeTool loads the tool definition from file and binds service.
// logger may be nil.
func NewCreateSubtypeTool(file string, service *servicelocation.Service, logger logger.Logger) (*CreateSubtypeTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &CreateSubtypeTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *CreateSubtypeTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that create_subtype is not safe to retry without journaling.
func (t *CreateSubtypeTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals create-subtype args, calls the service, and returns JSON output.
func (t *CreateSubtypeTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute create_subtype", "type", "CreateSubtypeTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.CreateSubtype(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute create_subtype", "type", "CreateSubtypeTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute create_subtype", "type", "CreateSubtypeTool", "func", "Execute", "error", err)
		return nil, err
	}

	info(t.logger, "create_subtype is executed", "type", "CreateSubtypeTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *CreateSubtypeTool) unmarshal(args json.RawMessage) (servicelocation.CreateSubtypeInput, error) {
	var result servicelocation.CreateSubtypeInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.CreateSubtypeInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *CreateSubtypeTool) marshal(output servicelocation.CreateSubtypeOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
