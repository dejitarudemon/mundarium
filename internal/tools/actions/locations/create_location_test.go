package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"gorm.io/gorm"
)

var (
	_ tools.Tool = (*toolslocations.CreateLocationTool)(nil)
)

func newExecCtx(t *testing.T) (execution.ExecutionContext[journal.Result], context.CancelFunc) {
	t.Helper()

	complete := func(context.Context, journal.ID, journal.ID, journal.Result, *gorm.DB) error {
		return nil
	}
	fail := func(context.Context, journal.ID, journal.ID, error, *gorm.DB) error {
		return nil
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		journal.ID("test-request"),
		journal.ID("test-group"),
		types.Role_Master,
		complete,
		fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func TestNewCreateLocationTool_NilService(t *testing.T) {
	_, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewCreateLocationTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewCreateLocationTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "create_location" {
		t.Fatalf("want definition name create_location, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "create_location" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if tool.IsIdempotent() {
		t.Fatal("create_location must not be idempotent")
	}
}

func TestCreateLocationTool_Execute_Root(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewCreateLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, json.RawMessage(`{
		"name":"World",
		"parent_id":null,
		"type":"world",
		"description":"prime",
		"details":{"tier":1}
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
		Name     string           `json:"name"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID == 0 || out.Name != "World" || out.ParentID != nil {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetLocation(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID != nil || got.Type != repolocations.LocationType_WORLD {
		t.Fatalf("unexpected persisted location: %+v", got)
	}
	if string(got.Details) != `{"tier":1}` {
		t.Fatalf("want details persisted, got %s", got.Details)
	}
}

func TestCreateLocationTool_Execute_Child(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "root",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}

	tool, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewCreateLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(map[string]any{
		"name":        "North",
		"parent_id":   root.ID,
		"type":        "region",
		"description": "cold",
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
		Name     string           `json:"name"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.Name != "North" || out.ParentID == nil || *out.ParentID != root.ID {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestCreateLocationTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewCreateLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestCreateLocationTool_Execute_ZeroParent(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewCreateLocationTool(createLocationDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewCreateLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{
		"name":"X",
		"parent_id":0,
		"type":"region"
	}`))
	if err == nil || !strings.Contains(err.Error(), "parent id is zero") {
		t.Fatalf("want parent id is zero, got %v", err)
	}
}
