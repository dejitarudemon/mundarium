package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	"github.com/google/uuid"
)

var (
	_ tools.Tool = (*toolslocations.CreateLocationTool)(nil)
)

func newExecCtx(t *testing.T) (execution.ExecutionContext, context.CancelFunc) {
	t.Helper()

	complete := func(context.Context, libjournal.RequestID, journal.Result) error { return nil }
	fail := func(context.Context, libjournal.RequestID, error) error { return nil }

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		libjournal.RequestID(uuid.New()),
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
	if !errors.Is(err, actions.ErrServiceIsNil) {
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
		ID       types.ID  `json:"id"`
		ParentID *types.ID `json:"parent_id"`
		Name     string    `json:"name"`
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
	if got.ParentID != nil || got.Type != types.LocationType_WORLD {
		t.Fatalf("unexpected persisted location: %+v", got)
	}
	if string(got.Details) != `{"tier":1}` {
		t.Fatalf("want details persisted, got %s", got.Details)
	}
}

func TestCreateLocationTool_Execute_Child(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: nil,
		Type:     types.LocationType_WORLD,
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
		ID       types.ID  `json:"id"`
		ParentID *types.ID `json:"parent_id"`
		Name     string    `json:"name"`
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
	if err == nil {
		t.Fatal("expected invalid json error")
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
	if err == nil || err.Error() != "parent id is zero" {
		t.Fatalf("want parent id is zero, got %v", err)
	}
}
