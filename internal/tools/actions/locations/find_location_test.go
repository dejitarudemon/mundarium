package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
)

var (
	_ tools.Tool = (*toolslocations.FindLocationsTool)(nil)
)

func TestNewFindLocationsTool_NilService(t *testing.T) {
	_, err := toolslocations.NewFindLocationsTool(findLocationsDefinitionPath(t), nil)
	if !errors.Is(err, actions.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewFindLocationsTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewFindLocationsTool(findLocationsDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewFindLocationsTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "find_locations" {
		t.Fatalf("want definition name find_locations, got %q", def.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("find_locations must be idempotent")
	}
}

func TestFindLocationsTool_Execute(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: nil,
		Type:     types.LocationType_WORLD,
		Name:     "World",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}
	north, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: &root.ID,
		Type:     types.LocationType_REGION,
		Name:     "North",
	})
	if err != nil {
		t.Fatalf("seed north: %v", err)
	}
	if _, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: &north.ID,
		Type:     types.LocationType_SITE,
		Name:     "North Gate",
	}); err != nil {
		t.Fatalf("seed gate: %v", err)
	}

	tool, err := toolslocations.NewFindLocationsTool(findLocationsDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewFindLocationsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(map[string]any{
		"scope_id": root.ID,
		"query":    "Gate",
		"depth":    2,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		Scope struct {
			Name string `json:"name"`
		} `json:"scope"`
		Children map[string][]struct {
			Name string `json:"name"`
		} `json:"children"`
		HasMore int `json:"has_more"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// JSON object keys are strings; depth 2 holds North Gate (North itself is filtered out by query)
	if out.Scope.Name != "World" || len(out.Children["2"]) != 1 || out.Children["2"][0].Name != "North Gate" || out.HasMore != 0 {
		t.Fatalf("want World scope + North Gate at depth 2, got %#v", out)
	}
}

func TestFindLocationsTool_Execute_ZeroScope(t *testing.T) {
	resetDB(t)

	tool, err := toolslocations.NewFindLocationsTool(findLocationsDefinitionPath(t), testSvc)
	if err != nil {
		t.Fatalf("NewFindLocationsTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"scope_id":0}`))
	if err == nil || err.Error() != "scope id is zero" {
		t.Fatalf("want scope id is zero, got %v", err)
	}
}
