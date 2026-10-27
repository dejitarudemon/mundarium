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
	_ tools.Tool = (*toolslocations.FindConnectionsTool)(nil)
)

func TestNewFindConnectionsTool_NilService(t *testing.T) {
	_, err := toolslocations.NewFindConnectionsTool(findConnectionsDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewFindConnectionsTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewFindConnectionsTool(findConnectionsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindConnectionsTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "find_connections" {
		t.Fatalf("want definition name find_connections, got %q", def.Name)
	}
	def.Name = "mutated"
	again := tool.Definition()
	if again.Name != "find_connections" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("find_connections must be idempotent")
	}
}

func TestFindConnectionsTool_Execute(t *testing.T) {
	resetDB(t)

	a := seedLocation(t, "A")
	b := seedLocation(t, "B")
	c := seedLocation(t, "C")
	ab := seedConnection(t, a.ID, b.ID, "ab", false)
	_ = seedConnection(t, a.ID, c.ID, "ac", false)
	_ = seedConnection(t, b.ID, a.ID, "ba", true)

	findTool, err := toolslocations.NewFindConnectionsTool(findConnectionsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindConnectionsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := findTool.Execute(ctx, mustJSON(t, map[string]any{
		"from_id": a.ID,
		"to_id":   b.ID,
	}))
	if err != nil {
		t.Fatalf("FindConnections directed: %v", err)
	}

	var out struct {
		Connections []struct {
			ID repositories.ID `json:"id"`
		} `json:"connections"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Connections) != 1 || out.Connections[0].ID != ab.ID {
		t.Fatalf("want only ab, got %#v", out.Connections)
	}

	either, err := findTool.Execute(ctx, mustJSON(t, map[string]any{
		"from_id":          a.ID,
		"to_id":            b.ID,
		"either_direction": true,
	}))
	if err != nil {
		t.Fatalf("FindConnections either: %v", err)
	}
	if err := json.Unmarshal(either, &out); err != nil {
		t.Fatalf("unmarshal either: %v", err)
	}
	if len(out.Connections) != 2 {
		t.Fatalf("want ab+ba, got %#v", out.Connections)
	}
}

func TestFindConnectionsTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewFindConnectionsTool(findConnectionsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindConnectionsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func seedConnection(t *testing.T, fromID, toID repositories.ID, description string, bidirectional bool) *repolocations.Connection {
	t.Helper()
	conn := &repolocations.Connection{
		FromID:        fromID,
		ToID:          toID,
		Description:   description,
		Bidirectional: bidirectional,
	}
	if err := testRepo.CreateConnection(context.Background(), conn); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	return conn
}
