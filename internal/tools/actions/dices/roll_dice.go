// Package dices implements dice-related tools.
package dices

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	servicedices "github.com/dejitarudemon/mundarium/internal/services/dices"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*RollDiceTool)(nil)
)

const diceEntity = "DICE"

// RollDiceTool rolls dice via the dices service.
type RollDiceTool struct {
	service    *servicedices.Service
	definition definition.Definition
	logger     logger.Logger
}

// providerDieResult is the minimal per-group payload returned to the provider.
type providerDieResult struct {
	Sides uint8  `json:"sides"`
	Rolls []uint `json:"rolls"`
}

// providerResult is the minimal roll payload returned to the provider.
type providerResult struct {
	Total int                 `json:"total"`
	Dice  []providerDieResult `json:"dice"`
}

// NewRollDiceTool loads the tool definition from file and binds service.
// logger may be nil.
func NewRollDiceTool(file string, service *servicedices.Service, logger logger.Logger) (*RollDiceTool, error) {
	if service == nil {
		return nil, toolerrs.ErrServiceIsNil
	}

	def, err := actions.LoadDefinition(file)
	if err != nil {
		return nil, err
	}

	return &RollDiceTool{service: service, definition: def, logger: logger}, nil
}

// Definition returns a copy of the loaded tool schema.
func (t *RollDiceTool) Definition() definition.Definition {
	return t.definition.Copy()
}

// IsIdempotent reports that roll_dice is not safe to retry without journaling.
func (t *RollDiceTool) IsIdempotent() bool {
	return false
}

// Execute unmarshals dice args, rolls, journals the minimal provider payload, and returns it.
func (t *RollDiceTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	throw, err := t.unmarshal(args)
	if err != nil {
		t.error("failed to execute roll_dice", "type", "RollDiceTool", "func", "Execute", "error", err)
		return nil, err
	}

	result := t.service.Roll(throw)
	out := toProviderResult(result)

	raw, err := t.marshal(out)
	if err != nil {
		t.error("failed to execute roll_dice", "type", "RollDiceTool", "func", "Execute", "error", err)
		t.fail(ctx, err)
		return nil, err
	}

	if err := ctx.Complete(
		journal.Result{
			Action: journal.Action_Create,
			Entity: diceEntity,
			Data:   json.RawMessage(raw),
		},
		nil,
	); err != nil {
		err = toolerrs.From(err)
		t.error("failed to execute roll_dice", "type", "RollDiceTool", "func", "Execute", "error", err)
		t.fail(ctx, err)
		return nil, err
	}

	t.info("roll_dice is executed", "type", "RollDiceTool", "func", "Execute", "data", result)
	return raw, nil
}

func (t *RollDiceTool) unmarshal(args json.RawMessage) (repodices.DicesGroup, error) {
	var result repodices.DicesGroup
	if err := json.Unmarshal(args, &result); err != nil {
		return repodices.DicesGroup{}, toolerrs.NewInvalidRequest(err)
	}
	if len(result.Dices) == 0 {
		return repodices.DicesGroup{}, toolerrs.NewInvalidRequest(errors.New("dieces_group must not be empty"))
	}
	for i, die := range result.Dices {
		if die.Sides == 0 {
			return repodices.DicesGroup{}, toolerrs.NewInvalidRequest(fmt.Errorf("dieces_group[%d].sides must be >= 1", i))
		}
		if die.Count == 0 {
			return repodices.DicesGroup{}, toolerrs.NewInvalidRequest(fmt.Errorf("dieces_group[%d].count must be >= 1", i))
		}
	}
	return result, nil
}

func (t *RollDiceTool) marshal(output providerResult) (json.RawMessage, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, toolerrs.NewInvalidRequest(err)
	}
	return raw, nil
}

func toProviderResult(result repodices.ThrowResult) providerResult {
	out := providerResult{
		Total: result.Total,
		Dice:  make([]providerDieResult, 0, len(result.Dices)),
	}
	for _, group := range result.Dices {
		rolls := group.Rolls
		if rolls == nil {
			rolls = []uint{}
		}
		out.Dice = append(out.Dice, providerDieResult{
			Sides: group.Die.Sides,
			Rolls: rolls,
		})
	}
	return out
}

func (t *RollDiceTool) fail(ctx execution.ExecutionContext[journal.Result], cause error) {
	if err := ctx.Fail(cause, nil); err != nil {
		t.warn("failed to record Fail in journal", "type", "RollDiceTool", "cause", cause, "error", err)
	}
}
