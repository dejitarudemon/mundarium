package events_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolsevents.CreateEventTool)(nil)
)

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func TestNewCreateEventTool_NilService(t *testing.T) {
	_, err := toolsevents.NewCreateEventTool(defPath(t, "create_event.json"), nil, nil)
	if !errors.Is(err, toolerrs.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestCreateEventTool_LoadsDefinition(t *testing.T) {
	tool, err := toolsevents.NewCreateEventTool(defPath(t, "create_event.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateEventTool: %v", err)
	}
	def := tool.Definition()
	if def.Name != "create_event" {
		t.Fatalf("want create_event, got %q", def.Name)
	}
	def.Name = "mutated"
	if tool.Definition().Name != "create_event" {
		t.Fatal("Definition must return a copy")
	}
	if tool.IsIdempotent() {
		t.Fatal("create_event must not be idempotent")
	}
}

func TestCreateEventTool_Execute(t *testing.T) {
	resetDB(t)

	cause, err := testRepo.CreateEvent(context.Background(), &repoevents.Event{
		Importance: 10, CreatedAtGameTime: 5, Summary: "prior-cause",
	})
	if err != nil {
		t.Fatalf("seed cause: %v", err)
	}
	entityID := entities.New()
	locID := repositories.ID(3)

	tool, err := toolsevents.NewCreateEventTool(defPath(t, "create_event.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateEventTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, mustJSON(t, map[string]any{
		"location_id":          locID,
		"importance":           70,
		"created_at_game_time": 10,
		"summary":              "effect",
		"tags":                 []string{"quest"},
		"entities": []map[string]any{
			{"entity_id": entityID.String(), "role": "actor"},
		},
		"causals": []map[string]any{
			{"from": cause.ID.String(), "cause": "leads to"},
		},
	}))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID      repoevents.ID `json:"id"`
		Summary string        `json:"summary"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Summary != "effect" || out.ID.String() == "" {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetEvent(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if len(got.Entities) != 1 {
		t.Fatalf("want nested entity, got %+v", got)
	}
	causeGot, err := testRepo.GetEvent(context.Background(), cause.ID)
	if err != nil {
		t.Fatalf("GetEvent cause: %v", err)
	}
	if len(causeGot.Causals) != 1 || causeGot.Causals[0].To != out.ID {
		t.Fatalf("want incoming causal from cause->effect, got %+v", causeGot.Causals)
	}
}

func TestCreateEventTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolsevents.NewCreateEventTool(defPath(t, "create_event.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateEventTool: %v", err)
	}
	ctx, cancel := newExecCtx(t)
	defer cancel()
	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest, got %v", err)
	}
}
