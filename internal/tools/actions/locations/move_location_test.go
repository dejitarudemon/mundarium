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
	_ tools.Tool = (*toolslocations.MoveLocationTool)(nil)
)

func TestNewMoveLocationTool_NilService(t *testing.T) {
	_, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewMoveLocationTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "move_location" {
		t.Fatalf("want definition name move_location, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "move_location" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("move_location must be idempotent")
	}
}

func TestMoveLocationTool_Execute(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "root",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}
	other, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "other",
	})
	if err != nil {
		t.Fatalf("seed other: %v", err)
	}
	child, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: &root.ID,
		Type:     repolocations.LocationType_REGION,
		Name:     "child",
	})
	if err != nil {
		t.Fatalf("seed child: %v", err)
	}

	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(toolslocations.MoveLocationArgs{
		ID:       child.ID,
		ParentID: &other.ID,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out toolslocations.MoveLocationResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != child.ID || out.ParentID == nil || *out.ParentID != other.ID {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetLocation(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID == nil || *got.ParentID != other.ID {
		t.Fatalf("want parent %d, got %v", other.ID, got.ParentID)
	}

	// Same move again must succeed (idempotent).
	raw2, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute retry: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Fatalf("retry output mismatch: %s vs %s", raw2, raw)
	}
}

func TestMoveLocationTool_Execute_ToRoot(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "root",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}
	child, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: &root.ID,
		Type:     repolocations.LocationType_REGION,
		Name:     "child",
	})
	if err != nil {
		t.Fatalf("seed child: %v", err)
	}

	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(toolslocations.MoveLocationArgs{
		ID:       child.ID,
		ParentID: nil,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out toolslocations.MoveLocationResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != child.ID || out.ParentID != nil {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetLocation(ctx, child.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if got.ParentID != nil {
		t.Fatalf("want root ParentID=nil, got %v", got.ParentID)
	}
}

func TestMoveLocationTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestMoveLocationTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":0,"parent_id":null}`))
	if !errors.Is(err, toolerrs.ErrLocationIDZero) {
		t.Fatalf("want ErrLocationIDZero, got %v", err)
	}
}

func TestMoveLocationTool_Execute_ZeroParent(t *testing.T) {
	tool, err := toolslocations.NewMoveLocationTool(moveLocationDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewMoveLocationTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":1,"parent_id":0}`))
	if !errors.Is(err, toolerrs.ErrParentIDZero) {
		t.Fatalf("want ErrParentIDZero, got %v", err)
	}
}
