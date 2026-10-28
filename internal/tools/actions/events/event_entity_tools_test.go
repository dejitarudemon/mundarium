package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolsevents.CreateEventEntityTool)(nil)
	_ tools.Tool = (*toolsevents.FindEventEntitiesTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventEntityRoleTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventEntityKnownAtTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventEntityBeliefTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventEntityDetailsTool)(nil)
)

func TestEventEntityTools_Flow(t *testing.T) {
	resetDB(t)

	ev, err := testRepo.CreateEvent(context.Background(), &repoevents.Event{
		Importance: 10, CreatedAtGameTime: 1, Summary: "meet",
	})
	if err != nil {
		t.Fatalf("seed event: %v", err)
	}
	entityID := entities.New()

	createTool, err := toolsevents.NewCreateEventEntityTool(defPath(t, "create_event_entity.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("create tool: %v", err)
	}
	findTool, err := toolsevents.NewFindEventEntitiesTool(defPath(t, "find_event_entities.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("find tool: %v", err)
	}
	roleTool, err := toolsevents.NewUpdateEventEntityRoleTool(defPath(t, "update_event_entity_role.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("role tool: %v", err)
	}
	knownTool, err := toolsevents.NewUpdateEventEntityKnownAtTool(defPath(t, "update_event_entity_known_at.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("known tool: %v", err)
	}
	beliefTool, err := toolsevents.NewUpdateEventEntityBeliefTool(defPath(t, "update_event_entity_belief.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("belief tool: %v", err)
	}
	detailsTool, err := toolsevents.NewUpdateEventEntityDetailsTool(defPath(t, "update_event_entity_details.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("details tool: %v", err)
	}

	if createTool.IsIdempotent() || knownTool.IsIdempotent() {
		t.Fatal("create/known_at must not be idempotent")
	}
	if !findTool.IsIdempotent() || !roleTool.IsIdempotent() || !beliefTool.IsIdempotent() || !detailsTool.IsIdempotent() {
		t.Fatal("find/role/belief/details must be idempotent")
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := createTool.Execute(ctx, mustJSON(t, map[string]any{
		"entity_id": entityID.String(),
		"event_id":  ev.ID.String(),
		"role":      "witness",
	}))
	if err != nil {
		t.Fatalf("create entity: %v", err)
	}
	var created struct {
		Role string `json:"role"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.Role != "witness" {
		t.Fatalf("unexpected create output: %s (%v)", raw, err)
	}

	if _, err := roleTool.Execute(ctx, mustJSON(t, map[string]any{
		"entity_id": entityID.String(), "event_id": ev.ID.String(), "role": "actor",
	})); err != nil {
		t.Fatalf("role: %v", err)
	}
	if _, err := knownTool.Execute(ctx, mustJSON(t, map[string]any{
		"entity_id": entityID.String(), "event_id": ev.ID.String(),
		"known_at_game_time": 5, "belief": 80,
	})); err != nil {
		t.Fatalf("known_at: %v", err)
	}
	if _, err := beliefTool.Execute(ctx, mustJSON(t, map[string]any{
		"entity_id": entityID.String(), "event_id": ev.ID.String(), "belief": 40,
	})); err != nil {
		t.Fatalf("belief: %v", err)
	}
	if _, err := detailsTool.Execute(ctx, mustJSON(t, map[string]any{
		"entity_id": entityID.String(), "event_id": ev.ID.String(),
		"details": map[string]any{"source": "rumor"},
	})); err != nil {
		t.Fatalf("details: %v", err)
	}

	foundRaw, err := findTool.Execute(ctx, mustJSON(t, map[string]any{
		"event_id": ev.ID.String(), "known_only": true,
	}))
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	var found struct {
		Entities []struct {
			EntityID string `json:"entity_id"`
			Role     string `json:"role"`
			Belief   *int8  `json:"belief"`
		} `json:"entities"`
	}
	if err := json.Unmarshal(foundRaw, &found); err != nil {
		t.Fatalf("unmarshal find: %v", err)
	}
	if len(found.Entities) != 1 || found.Entities[0].EntityID != entityID.String() || found.Entities[0].Role != "actor" {
		t.Fatalf("unexpected find: %#v", found.Entities)
	}
	if found.Entities[0].Belief == nil || *found.Entities[0].Belief != 40 {
		t.Fatalf("want belief 40, got %#v", found.Entities[0].Belief)
	}
}

func TestCreateEventEntityTool_NilService(t *testing.T) {
	_, err := toolsevents.NewCreateEventEntityTool(defPath(t, "create_event_entity.json"), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}
