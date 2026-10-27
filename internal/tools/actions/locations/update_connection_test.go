package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolslocations.UpdateConnectionTool)(nil)
)

func TestNewUpdateConnectionTool_NilService(t *testing.T) {
	_, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewUpdateConnectionTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "update_connection" {
		t.Fatalf("want definition name update_connection, got %q", def.Name)
	}
	def.Name = "mutated"
	again := tool.Definition()
	if again.Name != "update_connection" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("update_connection must be idempotent")
	}
}

func TestUpdateConnectionTool_Execute(t *testing.T) {
	resetDB(t)

	a := seedLocation(t, "A")
	b := seedLocation(t, "B")
	c := seedLocation(t, "C")
	expires := int64(100)
	created := seedConnection(t, a.ID, b.ID, "old", false)
	created.ExpiresAt = &expires
	if err := testRepo.PutConnection(context.Background(), created); err != nil {
		t.Fatalf("seed expires_at: %v", err)
	}

	updateTool, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args := mustJSON(t, map[string]any{
		"id":            created.ID,
		"to_id":         c.ID,
		"bidirectional": true,
		"description":   "updated",
		"state":         map[string]any{"open": false},
		"details":       map[string]any{"note": "x"},
		"expires_at":    nil,
	})
	raw, err := updateTool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out toolslocations.UpdateConnectionResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != created.ID {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetConnection(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ToID != c.ID || !got.Bidirectional || got.Description != "updated" || got.ExpiresAt != nil {
		t.Fatalf("unexpected updated connection: %+v", got)
	}

	// Same update again must succeed (idempotent).
	if _, err := updateTool.Execute(ctx, args); err != nil {
		t.Fatalf("Execute retry: %v", err)
	}
}

func TestUpdateConnectionTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"description":"x"}`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestUpdateConnectionTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestUpdateConnectionTool_Execute_NotFound(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewUpdateConnectionTool(updateConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, mustJSON(t, map[string]any{
		"id":            999,
		"to_id":         1,
		"bidirectional": false,
		"description":   "x",
		"state":         map[string]any{},
		"details":       map[string]any{},
		"expires_at":    nil,
	}))
	if err == nil {
		t.Fatal("expected not found error")
	}
}
