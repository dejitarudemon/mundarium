package events_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolsevents.FindEventsTool)(nil)
)

func TestFindEventsTool_Execute(t *testing.T) {
	resetDB(t)

	createTool, err := toolsevents.NewCreateEventTool(defPath(t, "create_event.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateEventTool: %v", err)
	}
	findTool, err := toolsevents.NewFindEventsTool(defPath(t, "find_events.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("NewFindEventsTool: %v", err)
	}
	if !findTool.IsIdempotent() {
		t.Fatal("find_events must be idempotent")
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	locA := repositories.ID(11)
	raw, err := createTool.Execute(ctx, mustJSON(t, map[string]any{
		"location_id":          locA,
		"importance":           40,
		"created_at_game_time": 1,
		"summary":              "at-a",
		"tags":                 []string{"sword"},
	}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}

	_, err = createTool.Execute(ctx, mustJSON(t, map[string]any{
		"location_id":          repositories.ID(22),
		"importance":           90,
		"created_at_game_time": 2,
		"summary":              "at-b",
		"tags":                 []string{"shield"},
	}))
	if err != nil {
		t.Fatalf("create b: %v", err)
	}

	foundRaw, err := findTool.Execute(ctx, mustJSON(t, map[string]any{
		"location_id": locA,
		"tags":        []string{"sword"},
	}))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	var found struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	if err := json.Unmarshal(foundRaw, &found); err != nil {
		t.Fatalf("unmarshal find: %v", err)
	}
	if len(found.Events) != 1 || found.Events[0].ID != created.ID {
		t.Fatalf("want only created event, got %#v", found.Events)
	}
}

func TestFindEventsTool_NilService(t *testing.T) {
	_, err := toolsevents.NewFindEventsTool(defPath(t, "find_events.json"), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}
