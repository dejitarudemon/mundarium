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
	_ tools.Tool = (*toolslocations.CreateSubtypeTool)(nil)
)

func TestNewCreateSubtypeTool_NilService(t *testing.T) {
	_, err := toolslocations.NewCreateSubtypeTool(createSubtypeDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewCreateSubtypeTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewCreateSubtypeTool(createSubtypeDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateSubtypeTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "create_subtype" {
		t.Fatalf("want definition name create_subtype, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "create_subtype" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if tool.IsIdempotent() {
		t.Fatal("create_subtype must not be idempotent")
	}
}

func TestCreateSubtypeTool_Execute(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewCreateSubtypeTool(createSubtypeDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateSubtypeTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, json.RawMessage(`{
		"key":"TaVeRn",
		"type":"structure",
		"defining_tags":["rest","sleep"],
		"expected_tags":["village"],
		"description":"used for safe, paid recreation"
	}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID  repositories.ID `json:"id"`
		Key string          `json:"key"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID == 0 || out.Key != "tavern" {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetSubtype(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("GetSubtype: %v", err)
	}
	if got.Key != "tavern" || got.Type != repolocations.LocationType_STRUCTURE {
		t.Fatalf("unexpected persisted subtype: %+v", got)
	}
	if len(got.DefiningTags) != 2 || got.DefiningTags[0] != "rest" {
		t.Fatalf("unexpected DefiningTags: %#v", got.DefiningTags)
	}
}

func TestCreateSubtypeTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewCreateSubtypeTool(createSubtypeDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateSubtypeTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestCreateSubtypeTool_Execute_EmptyKey(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewCreateSubtypeTool(createSubtypeDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateSubtypeTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{
		"key":"  ",
		"type":"structure",
		"defining_tags":[],
		"expected_tags":[],
		"description":"x"
	}`))
	if err == nil {
		t.Fatal("expected empty key error")
	}
}
