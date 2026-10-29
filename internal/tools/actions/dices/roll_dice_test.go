package dices_test

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	servicedices "github.com/dejitarudemon/mundarium/internal/services/dices"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsdices "github.com/dejitarudemon/mundarium/internal/tools/actions/dices"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"gorm.io/gorm"
)

var (
	_ tools.Tool = (*toolsdices.RollDiceTool)(nil)
)

func newExecCtx(t *testing.T) (execution.ExecutionContext[journal.Result], context.CancelFunc, *journal.Result) {
	t.Helper()

	var completed journal.Result
	complete := func(_ context.Context, _, _ journal.ID, result journal.Result, _ *gorm.DB) error {
		completed = result
		return nil
	}
	fail := func(context.Context, journal.ID, journal.ID, error, *gorm.DB) error {
		return nil
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		journal.ID("test-request"),
		journal.ID("inv-1"),
		types.Role_Master,
		complete,
		fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel, &completed
}

func definitionsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "definitions"))
}

func rollDiceDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "roll_dices.json")
}

func newTestService(t *testing.T, seed int64) *servicedices.Service {
	t.Helper()
	svc, err := servicedices.NewService(repodices.NewDicesRepository(rand.New(rand.NewSource(seed)), nil), nil)
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	return svc
}

func TestNewRollDiceTool_NilService(t *testing.T) {
	t.Parallel()

	_, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewRollDiceTool_LoadsDefinition(t *testing.T) {
	t.Parallel()

	tool, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), newTestService(t, 1), nil)
	if err != nil {
		t.Fatalf("NewRollDiceTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "roll_dice" {
		t.Fatalf("want definition name roll_dice, got %q", def.Name)
	}
	def.Name = "mutated"
	if tool.Definition().Name != "roll_dice" {
		t.Fatal("Definition must return a copy")
	}
	if tool.IsIdempotent() {
		t.Fatal("roll_dice must not be idempotent")
	}
}

func TestRollDiceTool_Execute(t *testing.T) {
	t.Parallel()

	tool, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), newTestService(t, 42), nil)
	if err != nil {
		t.Fatalf("NewRollDiceTool: %v", err)
	}

	ctx, cancel, completed := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, json.RawMessage(`{
		"dieces_group": [
			{"sides": 8, "count": 1},
			{"sides": 6, "count": 3}
		]
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		Total int `json:"total"`
		Dice  []struct {
			Sides uint8  `json:"sides"`
			Rolls []uint `json:"rolls"`
		} `json:"dice"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if len(out.Dice) != 2 {
		t.Fatalf("dice groups=%d, want 2", len(out.Dice))
	}
	if out.Dice[0].Sides != 8 || len(out.Dice[0].Rolls) != 1 {
		t.Fatalf("first group unexpected: %+v", out.Dice[0])
	}
	if out.Dice[1].Sides != 6 || len(out.Dice[1].Rolls) != 3 {
		t.Fatalf("second group unexpected: %+v", out.Dice[1])
	}
	for _, face := range out.Dice[0].Rolls {
		if face < 1 || face > 8 {
			t.Fatalf("d8 face %d out of range", face)
		}
	}
	for _, face := range out.Dice[1].Rolls {
		if face < 1 || face > 6 {
			t.Fatalf("d6 face %d out of range", face)
		}
	}
	if out.Total == 0 {
		t.Fatal("total must be non-zero for this roll")
	}

	// Provider payload must stay minimal: no die config fields.
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if _, ok := probe["dice"]; !ok {
		t.Fatal(`want "dice" key`)
	}
	if _, ok := probe["dices"]; ok {
		t.Fatal(`must not expose "dices"`)
	}

	if completed.Action != journal.Action_Create || completed.Entity != "DICE" {
		t.Fatalf("journal result: %+v", completed)
	}
	rawJournal, ok := completed.Data.(json.RawMessage)
	if !ok {
		t.Fatalf("journal data type %T, want json.RawMessage", completed.Data)
	}
	if string(rawJournal) != string(raw) {
		t.Fatalf("journal data mismatch:\n got %s\nwant %s", rawJournal, raw)
	}
}

func TestRollDiceTool_Execute_InvalidJSON(t *testing.T) {
	t.Parallel()

	tool, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), newTestService(t, 1), nil)
	if err != nil {
		t.Fatalf("NewRollDiceTool: %v", err)
	}

	ctx, cancel, _ := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestRollDiceTool_Execute_EmptyGroup(t *testing.T) {
	t.Parallel()

	tool, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), newTestService(t, 1), nil)
	if err != nil {
		t.Fatalf("NewRollDiceTool: %v", err)
	}

	ctx, cancel, _ := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"dieces_group":[]}`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestRollDiceTool_Execute_ZeroSides(t *testing.T) {
	t.Parallel()

	tool, err := toolsdices.NewRollDiceTool(rollDiceDefinitionPath(t), newTestService(t, 1), nil)
	if err != nil {
		t.Fatalf("NewRollDiceTool: %v", err)
	}

	ctx, cancel, _ := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"dieces_group":[{"sides":0,"count":1}]}`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}
