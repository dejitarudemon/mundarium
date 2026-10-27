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
	_ tools.Tool = (*toolslocations.UpdateLocationDetailsTool)(nil)
)

func TestNewUpdateLocationDetailsTool_NilService(t *testing.T) {
	_, err := toolslocations.NewUpdateLocationDetailsTool(updateLocationDetailsDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewUpdateLocationDetailsTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDetailsTool(updateLocationDetailsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDetailsTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "update_location_details" {
		t.Fatalf("want definition name update_location_details, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "update_location_details" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("update_location_details must be idempotent")
	}
}

func TestUpdateLocationDetailsTool_Execute(t *testing.T) {
	resetDB(t)

	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "place",
		Details:  repositories.JSON(`{"old":true,"keep":1}`),
	})
	if err != nil {
		t.Fatalf("seed location: %v", err)
	}

	tool, err := toolslocations.NewUpdateLocationDetailsTool(updateLocationDetailsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDetailsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	newDetails := repositories.JSON(`{"atmosphere":"quiet","keeper":"Marta"}`)
	args, err := json.Marshal(toolslocations.UpdateLocationDetailsArgs{
		ID:      loc.ID,
		Details: newDetails,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out toolslocations.UpdateLocationDetailsResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID != loc.ID || string(out.Details) != string(newDetails) {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetLocation(ctx, loc.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if string(got.Details) != string(newDetails) {
		t.Fatalf("want details fully replaced with %s, got %s", newDetails, got.Details)
	}

	// Same update again must succeed (idempotent).
	raw2, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute retry: %v", err)
	}
	if string(raw2) != string(raw) {
		t.Fatalf("retry output mismatch: %s vs %s", raw2, raw)
	}
}

func TestUpdateLocationDetailsTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDetailsTool(updateLocationDetailsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDetailsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestUpdateLocationDetailsTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewUpdateLocationDetailsTool(updateLocationDetailsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateLocationDetailsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":0,"details":{}}`))
	if !errors.Is(err, toolerrs.ErrLocationIDZero) {
		t.Fatalf("want ErrLocationIDZero, got %v", err)
	}
}
