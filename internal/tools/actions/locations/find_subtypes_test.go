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
	"github.com/lib/pq"
)

var (
	_ tools.Tool = (*toolslocations.FindSubtypesTool)(nil)
)

func TestNewFindSubtypesTool_NilService(t *testing.T) {
	_, err := toolslocations.NewFindSubtypesTool(findSubtypesDefinitionPath(t), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewFindSubtypesTool_LoadsDefinition(t *testing.T) {
	tool, err := toolslocations.NewFindSubtypesTool(findSubtypesDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindSubtypesTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "find_subtypes" {
		t.Fatalf("want definition name find_subtypes, got %q", def.Name)
	}
	def.Name = "mutated"
	if len(def.Parameters) > 0 {
		def.Parameters[0] ^= 0xff
	}
	again := tool.Definition()
	if again.Name != "find_subtypes" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if !tool.IsIdempotent() {
		t.Fatal("find_subtypes must be idempotent")
	}
}

func TestFindSubtypesTool_Execute(t *testing.T) {
	resetDB(t)

	tavern, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"},
		ExpectedTags: pq.StringArray{"village"},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	if _, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          "castle",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{},
		ExpectedTags: pq.StringArray{},
		Description:  "fort",
	}); err != nil {
		t.Fatalf("seed castle: %v", err)
	}
	if _, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          "cave",
		Type:         repolocations.LocationType_ROOM,
		DefiningTags: pq.StringArray{},
		ExpectedTags: pq.StringArray{},
		Description:  "dark",
	}); err != nil {
		t.Fatalf("seed cave: %v", err)
	}

	tool, err := toolslocations.NewFindSubtypesTool(findSubtypesDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindSubtypesTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	args, err := json.Marshal(map[string]any{
		"keys": []string{"tav", "cas"},
		"type": "structure",
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	raw, err := tool.Execute(ctx, args)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		Subtypes []struct {
			ID  repositories.ID `json:"id"`
			Key string          `json:"key"`
		} `json:"subtypes"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.Subtypes) != 2 {
		t.Fatalf("want 2 subtypes, got %#v", out.Subtypes)
	}

	idArgs, err := json.Marshal(map[string]any{"id": tavern.ID})
	if err != nil {
		t.Fatalf("marshal id args: %v", err)
	}
	byID, err := tool.Execute(ctx, idArgs)
	if err != nil {
		t.Fatalf("Execute by id: %v", err)
	}
	if err := json.Unmarshal(byID, &out); err != nil {
		t.Fatalf("unmarshal by id: %v", err)
	}
	if len(out.Subtypes) != 1 || out.Subtypes[0].ID != tavern.ID || out.Subtypes[0].Key != "tavern" {
		t.Fatalf("want only tavern, got %#v", out.Subtypes)
	}
}

func TestFindSubtypesTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolslocations.NewFindSubtypesTool(findSubtypesDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindSubtypesTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}

func TestFindSubtypesTool_Execute_ZeroID(t *testing.T) {
	tool, err := toolslocations.NewFindSubtypesTool(findSubtypesDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindSubtypesTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":0}`))
	if err == nil {
		t.Fatal("expected zero id error")
	}
}
