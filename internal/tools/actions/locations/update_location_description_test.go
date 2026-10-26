package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolslocations.UpdateLocationDescriptionTool)(nil)
)

func TestNewUpdateLocationDescriptionTool_NilService(t *testing.T) {
	_, err := toolslocations.NewUpdateLocationDescriptionTool(updateLocationDescriptionDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewUpdateLocationDescriptionTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDescriptionTool(updateLocationDescriptionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDescriptionTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "update_location_description" {
		t.Fatalf("want definition name update_location_description, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "update_location_description" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("update_location_description must be idempotent")
	}
}

func TestUpdateLocationDescriptionTool_Execute(t *testing.T) {
	resetDB(t)

	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_WORLD,
		Name:        "place",
		Description: "old description",
	})
	if err != nil {
		t.Fatalf("seed location: %v", err)
	}

	tool, err := toolslocations.NewUpdateLocationDescriptionTool(updateLocationDescriptionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDescriptionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	const newDesc = "updated narrative description"
	args, err := json.Marshal(toolslocations.UpdateLocationDescriptionArgs{
		ID:          loc.ID,
		Description: newDesc,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out toolslocations.UpdateLocationDescriptionResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != loc.ID || out.Description != newDesc {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.Description != newDesc {
		t.Fatalf("want description %q, got %q", newDesc, got.Description)
	}

	raw2, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute retry: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Fatalf("retry output mismatch: %s vs %s", raw2, raw)
	}
}

func TestUpdateLocationDescriptionTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDescriptionTool(updateLocationDescriptionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDescriptionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestUpdateLocationDescriptionTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDescriptionTool(updateLocationDescriptionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDescriptionTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":0,"description":"x"}`))
	if !errors.Is(err, toolerrs.ErrLocationIDZero) {
		t.Fatalf("want ErrLocationIDZero, got %v", err)
	}
}
