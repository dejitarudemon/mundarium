package events_test

import (
	"context"
	"encoding/json"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/google/uuid"
)

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE event_causals, event_entities, events, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func execute(t *testing.T, tool string, args any) (json.RawMessage, error) {
	t.Helper()
	return executeNamed(t, uuid.NewString(), tool+"-"+uuid.NewString()[:8], tool, args)
}

func executeNamed(t *testing.T, invokeID, callID, tool string, args any) (json.RawMessage, error) {
	t.Helper()
	return ex.Execute(context.Background(), invokeID, tools.Call{
		ID:   callID,
		Name: tool,
		Role: types.Role_Master,
		Args: mustJSON(t, args),
	})
}

func mustExecute(t *testing.T, tool string, args any) json.RawMessage {
	t.Helper()
	raw, err := execute(t, tool, args)
	if err != nil {
		t.Fatalf("%s: %v (raw=%s)", tool, err, raw)
	}
	return raw
}

func createEventViaTool(t *testing.T, args map[string]any) repoevents.ID {
	t.Helper()
	raw := mustExecute(t, "create_event", args)
	var out struct {
		ID repoevents.ID `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal create_event: %v raw=%s", err, raw)
	}
	if uuid.UUID(out.ID) == uuid.Nil {
		t.Fatalf("create_event returned nil id: %s", raw)
	}
	return out.ID
}

func findEvents(t *testing.T, args map[string]any) []map[string]any {
	t.Helper()
	raw := mustExecute(t, "find_events", args)
	var out struct {
		Events []map[string]any `json:"events"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal find_events: %v raw=%s", err, raw)
	}
	return out.Events
}

func findCausals(t *testing.T, args map[string]any) []map[string]any {
	t.Helper()
	raw := mustExecute(t, "find_event_causals", args)
	var out struct {
		Causals []map[string]any `json:"causals"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal find_event_causals: %v raw=%s", err, raw)
	}
	return out.Causals
}

func findEntities(t *testing.T, args map[string]any) []map[string]any {
	t.Helper()
	raw := mustExecute(t, "find_event_entities", args)
	var out struct {
		Entities []map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal find_event_entities: %v raw=%s", err, raw)
	}
	return out.Entities
}

func getEventDB(t *testing.T, id repoevents.ID) *repoevents.Event {
	t.Helper()
	ev, err := testRepo.GetEvent(context.Background(), id)
	if err != nil {
		t.Fatalf("GetEvent %s: %v", id, err)
	}
	return ev
}

func countEvents(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.Event{}).Count(&n).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func countCausals(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.EventCausal{}).Count(&n).Error; err != nil {
		t.Fatalf("count causals: %v", err)
	}
	return n
}

func countEntities(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.EventEntity{}).Count(&n).Error; err != nil {
		t.Fatalf("count entities: %v", err)
	}
	return n
}

func newEntityID() entities.ID {
	return entities.New()
}

func assertJournalFailed(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if req.Tool != tool {
		t.Fatalf("journal tool: want %s, got %s", tool, req.Tool)
	}
	if req.Status != journal.Status_Failed {
		t.Fatalf("journal status: want %s, got %s (error=%v)", journal.Status_Failed, req.Status, req.Error)
	}
	if req.Error.Err() == nil {
		t.Fatal("failed journal row must store error")
	}
}

func eventIDString(id repoevents.ID) string {
	return id.String()
}
