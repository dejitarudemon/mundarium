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
	_ tools.Tool = (*FindSubtypesTool)(nil)
)

// FindSubtypesTool searches location subtypes via the locations service.
type FindSubtypesTool struct {
	service    *servicelocation.Service
	definition definition.Definition
	logger     logger.Logger
}

// NewFindSubtypesTool loads the tool definition from file and binds service.
// logger may be nil.
func NewFindSubtypesTool(file string, service *servicelocation.Service, logger logger.Logger) (*FindSubtypesTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &FindSubtypesTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *FindSubtypesTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that find_subtypes is safe to retry without journaling.
func (t *FindSubtypesTool) IsIdempotent() bool {
	return true
}

// Execute unmarshals find-subtypes args, calls the service, and returns JSON output.
func (t *FindSubtypesTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	in, err := t.unmarshal(args)
	if err != nil {
		errorf(t.logger, "failed to execute find_subtypes", "type", "FindSubtypesTool", "func", "Execute", "error", err)
		return nil, err
	}

	result, err := t.service.FindSubtypes(ctx, in)
	if err != nil {
		err = toolerrs.From(err)
		errorf(t.logger, "failed to execute find_subtypes", "type", "FindSubtypesTool", "func", "Execute", "error", err)
		return nil, err
	}

	raw, err := t.marshal(result)
	if err != nil {
		errorf(t.logger, "failed to execute find_subtypes", "type", "FindSubtypesTool", "func", "Execute", "error", err)
		return nil, err
	}

	debug(t.logger, "find_subtypes is executed", "type", "FindSubtypesTool", "func", "Execute", "count", len(result.Subtypes))
	return raw, nil
}

func (t *FindSubtypesTool) unmarshal(args json.RawMessage) (servicelocation.FindSubtypesInput, error) {
	var result servicelocation.FindSubtypesInput
	if err := json.Unmarshal(args, &result); err != nil {
		return servicelocation.FindSubtypesInput{}, toolerrs.NewInvalidRequest(err)
	}

	return result, nil
}

func (t *FindSubtypesTool) marshal(output servicelocation.FindSubtypesOutput) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}
