package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"gorm.io/gorm"
)

var (
	_ tools.Tool = (*toolslocations.DeleteConnectionTool)(nil)
)

func TestNewDeleteConnectionTool_NilService(t *testing.T) {
	_, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewDeleteConnectionTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewDeleteConnectionTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "delete_connection" {
		t.Fatalf("want definition name delete_connection, got %q", def.Name)
	}
	def.Name = "mutated"
	again := tool.Definition()
	if again.Name != "delete_connection" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if tool.IsIdempotent() {
		t.Fatal("delete_connection must not be idempotent")
	}
}

func TestDeleteConnectionTool_Execute(t *testing.T) {
	resetDB(t)

	from := seedLocation(t, "from")
	to := seedLocation(t, "to")
	conn := seedConnection(t, from.ID, to.ID, "doomed", false)

	tool, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewDeleteConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, mustJSON(t, map[string]any{"id": conn.ID}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID repositories.ID `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != conn.ID {
		t.Fatalf("want id %d, got %d", conn.ID, out.ID)
	}

	_, err = testRepo.GetConnection(context.Background(), conn.ID)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("want ErrRecordNotFound after delete, got %v", err)
	}
}

func TestDeleteConnectionTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewDeleteConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, mustJSON(t, map[string]any{"id": 0}))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestDeleteConnectionTool_Execute_NotFound(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewDeleteConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, mustJSON(t, map[string]any{"id": 999}))
	if err == nil {
		t.Fatal("expected not found error")
	}
}

func TestDeleteConnectionTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewDeleteConnectionTool(deleteConnectionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewDeleteConnectionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}
