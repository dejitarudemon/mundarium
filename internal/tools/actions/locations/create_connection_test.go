package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolslocations.CreateConnectionTool)(nil)
)

func TestNewCreateConnectionTool_NilService(t *testing.T) {
	_, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewCreateConnectionTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateConnectionTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "create_connection" {
		t.Fatalf("want definition name create_connection, got %q", def.Name)
	}
	def.Name = "mutated"
	again := tool.Definition()
	if again.Name != "create_connection" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if tool.IsIdempotent() {
		t.Fatal("create_connection must not be idempotent")
	}
}

func TestCreateConnectionTool_Execute(t *testing.T) {
	resetDB(t)

	from := seedLocation(t, "from")
	to := seedLocation(t, "to")

	tool, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"from_id":       from.ID,
		"to_id":         to.ID,
		"bidirectional": true,
		"description":   "secret tunnel",
		"state":         map[string]any{"open": true},
		"details":       map[string]any{"hidden": true},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID     repositories.ID `json:"id"`
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID == 0 || out.FromID != from.ID || out.ToID != to.ID {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetConnection(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if !got.Bidirectional || got.Description != "secret tunnel" {
		t.Fatalf("unexpected persisted connection: %+v", got)
	}
}

func TestCreateConnectionTool_Execute_SelfLink(t *testing.T) {
	resetDB(t)
	loc := seedLocation(t, "loop")

	tool, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(map[string]any{
		"from_id":     loc.ID,
		"to_id":       loc.ID,
		"description": "magical corridor",
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.FromID != out.ToID {
		t.Fatalf("want self-link, got %+v", out)
	}
}

func TestCreateConnectionTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestCreateConnectionTool_Execute_MissingEndpoint(t *testing.T) {
	resetDB(t)
	from := seedLocation(t, "from")

	tool, err := toolslocations.NewCreateConnectionTool(createConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(map[string]any{
		"from_id":     from.ID,
		"to_id":       999,
		"description": "broken",
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	_, err = tool.Execute(ctx, args)
	if err == nil {
		t.Fatal("expected FK error")
	}
}

func seedLocation(t *testing.T, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		Type: repolocations.LocationType_REGION,
		Name: name,
	})
	if err != nil {
		t.Fatalf("seed location %q: %v", name, err)
	}
	return loc
}
