package executor_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	libtools "github.com/dejitarudemon/mundarium/internal/libs/tools"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"golang.org/x/crypto/blake2b"
)

func mustHashArgs(t *testing.T, args json.RawMessage) []byte {
	t.Helper()
	sum := blake2b.Sum256(args)
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

func newExecutor(t *testing.T, j journal.Journal, tool tools.Tool) *executor.Executor {
	t.Helper()

	if j == nil {
		j = implemented.NewMapJournal()
	}

	ex, err := executor.NewExecutor(j)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	if tool != nil {
		if err := ex.Register(tool); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	return ex
}

func TestExecute_ToolNotFound(t *testing.T) {
	ex := newExecutor(t, nil, nil)
	_, err := ex.Execute(context.Background(), "inv-1", libtools.Call{ID: "c1", Name: "missing"})
	if !errors.Is(err, executor.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestExecuteNotIdempotent_CreateRoot(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	got, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{
			"name":"World",
			"parent_id":null,
			"type":"world",
			"description":"prime",
			"details":{"tier":1}
		}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       types.ID  `json:"id"`
		ParentID *types.ID `json:"parent_id"`
		Name     string    `json:"name"`
	}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.ID == 0 || out.Name != "World" || out.ParentID != nil {
		t.Fatalf("unexpected output: %+v", out)
	}

	loc, err := testRepo.GetLocation(context.Background(), out.ID)
	if err != nil {
		t.Fatalf("GetLocation: %v", err)
	}
	if loc.ParentID != nil || loc.Type != types.LocationType_WORLD {
		t.Fatalf("unexpected persisted location: %+v", loc)
	}
	if string(loc.Details) != `{"tier":1}` {
		t.Fatalf("want details persisted, got %s", loc.Details)
	}
}

func TestExecuteNotIdempotent_CreateChild(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: nil,
		Type:     types.LocationType_WORLD,
		Name:     "root",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}

	args, err := json.Marshal(map[string]any{
		"name":        "North",
		"parent_id":   root.ID,
		"type":        "region",
		"description": "cold",
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	got, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: args,
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       types.ID  `json:"id"`
		ParentID *types.ID `json:"parent_id"`
		Name     string    `json:"name"`
	}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Name != "North" || out.ParentID == nil || *out.ParentID != root.ID {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestExecuteNotIdempotent_ReplayCompleted(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	call := libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`),
	}

	first, err := ex.Execute(context.Background(), "inv-1", call)
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	second, err := ex.Execute(context.Background(), "inv-1", call)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("replay mismatch:\nfirst=%s\nsecond=%s", first, second)
	}

	var count int64
	if err := testConn.DB().Table("locations").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("completed request must not create twice, count=%d", count)
	}
}

func TestExecuteNotIdempotent_StillRunning(t *testing.T) {
	resetDB(t)

	j := implemented.NewMapJournal()
	ex := newExecutor(t, j, newCreateLocationTool(t))

	args := json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`)
	requestID := libjournal.RequestID("inv-1:call-1")
	if _, _, err := j.Register(context.Background(), requestID, "create_location", mustHashArgs(t, args)); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: args,
	})
	if !errors.Is(err, executor.ErrRequestStillRunning) {
		t.Fatalf("want ErrRequestStillRunning, got %v", err)
	}

	var count int64
	if err := testConn.DB().Table("locations").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("running request must not execute, count=%d", count)
	}
}

func TestExecuteNotIdempotent_ToolMismatch(t *testing.T) {
	resetDB(t)

	j := implemented.NewMapJournal()
	ex := newExecutor(t, j, newCreateLocationTool(t))

	args := json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`)
	requestID := libjournal.RequestID("inv-1:call-1")
	if _, _, err := j.Register(context.Background(), requestID, "other_tool", mustHashArgs(t, args)); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: args,
	})
	if !errors.Is(err, executor.ErrRequestToolMismatch) {
		t.Fatalf("want ErrRequestToolMismatch, got %v", err)
	}
}

func TestExecuteNotIdempotent_ArgsMismatch(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	first := libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`),
	}
	if _, err := ex.Execute(context.Background(), "inv-1", first); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{"name":"Other","parent_id":null,"type":"world"}`),
	})
	if !errors.Is(err, executor.ErrRequestArgsMismatch) {
		t.Fatalf("want ErrRequestArgsMismatch, got %v", err)
	}

	var count int64
	if err := testConn.DB().Table("locations").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("args mismatch must not create another row, count=%d", count)
	}
}

func TestExecuteNotIdempotent_AlreadyFailed(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	call := libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{"name":"X","parent_id":0,"type":"region"}`),
	}

	_, err := ex.Execute(context.Background(), "inv-1", call)
	if err == nil || err.Error() != "parent id is zero" {
		t.Fatalf("want parent id is zero, got %v", err)
	}

	_, err = ex.Execute(context.Background(), "inv-1", call)
	if err == nil || err.Error() != "parent id is zero" {
		t.Fatalf("want replayed parent id is zero, got %v", err)
	}

	var count int64
	if err := testConn.DB().Table("locations").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("failed request must not persist rows, count=%d", count)
	}
}
