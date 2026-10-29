package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolsevents.CreateEventCausalTool)(nil)
	_ tools.Tool = (*toolsevents.FindEventCausalsTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventCausalCauseTool)(nil)
)

func TestEventCausalTools_Flow(t *testing.T) {
	resetDB(t)

	a, err := testRepo.CreateEvent(context.Background(), &repoevents.Event{
		Importance: 10, CreatedAtGameTime: 1, Summary: "a",
	})
	if err != nil {
		t.Fatalf("seed a: %v", err)
	}
	b, err := testRepo.CreateEvent(context.Background(), &repoevents.Event{
		Importance: 10, CreatedAtGameTime: 2, Summary: "b",
	})
	if err != nil {
		t.Fatalf("seed b: %v", err)
	}

	createTool, err := toolsevents.NewCreateEventCausalTool(defPath(t, "create_event_causal.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("create tool: %v", err)
	}
	findTool, err := toolsevents.NewFindEventCausalsTool(defPath(t, "find_event_causals.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("find tool: %v", err)
	}
	patchTool, err := toolsevents.NewUpdateEventCausalCauseTool(defPath(t, "update_event_causal_cause.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("patch tool: %v", err)
	}

	if createTool.IsIdempotent() {
		t.Fatal("create_event_causal must not be idempotent")
	}
	if !findTool.IsIdempotent() || !patchTool.IsIdempotent() {
		t.Fatal("find/patch must be idempotent")
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	if _, err := createTool.Execute(ctx, mustJSON(t, map[string]any{
		"from": a.ID.String(), "to": b.ID.String(), "cause": "ab",
	})); err != nil {
		t.Fatalf("create causal: %v", err)
	}

	// Self-link rejected.
	_, err = createTool.Execute(ctx, mustJSON(t, map[string]any{
		"from": a.ID.String(), "to": a.ID.String(), "cause": "loop",
	}))
	if err == nil {
		t.Fatal("want error for self causal")
	}

	if _, err := patchTool.Execute(ctx, mustJSON(t, map[string]any{
		"from": a.ID.String(), "to": b.ID.String(), "cause": "therefore",
	})); err != nil {
		t.Fatalf("patch cause: %v", err)
	}

	foundRaw, err := findTool.Execute(ctx, mustJSON(t, map[string]any{
		"from": a.ID.String(), "to": b.ID.String(),
	}))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	var found struct {
		Causals []struct {
			Cause string `json:"cause"`
		} `json:"causals"`
	}
	if err := json.Unmarshal(foundRaw, &found); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(found.Causals) != 1 || found.Causals[0].Cause != "therefore" {
		t.Fatalf("unexpected causals: %#v", found.Causals)
	}
}

func TestCreateEventCausalTool_NilService(t *testing.T) {
	_, err := toolsevents.NewCreateEventCausalTool(defPath(t, "create_event_causal.json"), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}
