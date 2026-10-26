package executor_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	"github.com/dejitarudemon/mundarium/internal/providers"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"golang.org/x/crypto/blake2b"
)

func masterCall(id, name string, args json.RawMessage) tools.Call {
	return tools.Call{
		ID:   id,
		Name: name,
		Role: providers.Role_Master,
		Args: args,
	}
}

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
		if err := ex.Register(tool, []providers.Role{providers.Role_Master}); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	return ex
}

func TestExecute_ToolNotFound(t *testing.T) {
	ex := newExecutor(t, nil, nil)
	_, err := ex.Execute(context.Background(), "inv-1", masterCall("c1", "missing", nil))
	if !errors.Is(err, executor.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestExecute_PermissionDenied(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	_, err := ex.Execute(context.Background(), "inv-1", tools.Call{
		ID:   "c1",
		Name: "create_location",
		Role: providers.Role("guest"),
		Args: json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`),
	})
	if !errors.Is(err, executor.ErrPermissionDenied) {
		t.Fatalf("want ErrPermissionDenied, got %v", err)
	}
}

func TestAllowed(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	defs := ex.Allowed(providers.Role_Master)
	if len(defs) != 1 || defs[0].Name != "create_location" {
		t.Fatalf("want create_location for master, got %#v", defs)
	}
	origParams := string(defs[0].Parameters)
	defs[0].Name = "mutated"
	if len(defs[0].Parameters) > 0 {
		defs[0].Parameters[0] ^= 0xff
	}
	again := ex.Allowed(providers.Role_Master)
	if len(again) != 1 || again[0].Name != "create_location" {
		t.Fatalf("Allowed must return a copy, got %#v", again)
	}
	if string(again[0].Parameters) != origParams {
		t.Fatalf("Allowed must clone Parameters, got %s want %s", again[0].Parameters, origParams)
	}
	if got := ex.Allowed(providers.Role("guest")); len(got) != 0 {
		t.Fatalf("want no tools for guest, got %#v", got)
	}
}

func TestExecuteIdempotent_FindLocations(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "World",
	})
	if err != nil {
		t.Fatalf("seed root: %v", err)
	}

	ex := newExecutor(t, nil, newFindLocationsTool(t))
	args, err := json.Marshal(map[string]any{
		"scope_id": root.ID,
		"depth":    0,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	got, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "find_locations", args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		Scope struct {
			Name string `json:"name"`
		} `json:"scope"`
	}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Scope.Name != "World" {
		t.Fatalf("want World scope, got %#v", out)
	}
}

func TestExecuteNotIdempotent_CreateRoot(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	got, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "create_location", json.RawMessage(`{
			"name":"World",
			"parent_id":null,
			"type":"world",
			"description":"prime",
			"details":{"tier":1}
		}`)))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
		Name     string           `json:"name"`
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
	if loc.ParentID != nil || loc.Type != repolocations.LocationType_WORLD {
		t.Fatalf("unexpected persisted location: %+v", loc)
	}
	if string(loc.Details) != `{"tier":1}` {
		t.Fatalf("want details persisted, got %s", loc.Details)
	}
}

func TestExecuteNotIdempotent_CreateChild(t *testing.T) {
	resetDB(t)

	root, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
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
	got, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "create_location", args))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
		Name     string           `json:"name"`
	}
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Name != "North" || out.ParentID == nil || *out.ParentID != root.ID {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestExecuteNotIdempotent_SameIDsReplay(t *testing.T) {
	resetDB(t)

	tool := newCreateLocationTool(t)
	if tool.IsIdempotent() {
		t.Fatal("create_location must be non-idempotent")
	}

	j := implemented.NewMapJournal()
	ex := newExecutor(t, j, tool)

	const (
		invokeID = "inv-1"
		callID   = "call-1"
	)
	call := masterCall(callID, "create_location", json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`))

	first, err := ex.Execute(context.Background(), invokeID, call)
	if err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	req, err := j.Find(context.Background(), callID, invokeID)
	if err != nil {
		t.Fatalf("Find after first: %v", err)
	}
	if req.Status != journal.Status_Completed {
		t.Fatalf("after first Execute want completed, got %s", req.Status)
	}

	// Same invokeID + call.ID: journal hits AlreadyRegistered and returns the stored result.
	second, err := ex.Execute(context.Background(), invokeID, call)
	if err != nil {
		t.Fatalf("second Execute with same ids: %v", err)
	}
	if string(first) != string(second) {
		t.Fatalf("replay mismatch:\nfirst=%s\nsecond=%s", first, second)
	}

	var count int64
	if err := testConn.DB().Table("locations").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("same ids must not create twice, count=%d", count)
	}
}

func TestExecuteNotIdempotent_StillRunning(t *testing.T) {
	resetDB(t)

	j := implemented.NewMapJournal()
	ex := newExecutor(t, j, newCreateLocationTool(t))

	args := json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`)
	if _, _, err := j.Register(context.Background(), "call-1", "inv-1", "create_location", mustHashArgs(t, args)); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "create_location", args))
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
	if _, _, err := j.Register(context.Background(), "call-1", "inv-1", "other_tool", mustHashArgs(t, args)); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "create_location", args))
	if !errors.Is(err, executor.ErrRequestToolMismatch) {
		t.Fatalf("want ErrRequestToolMismatch, got %v", err)
	}
}

func TestExecuteNotIdempotent_ArgsMismatch(t *testing.T) {
	resetDB(t)

	ex := newExecutor(t, nil, newCreateLocationTool(t))
	first := masterCall("call-1", "create_location", json.RawMessage(`{"name":"World","parent_id":null,"type":"world"}`))
	if _, err := ex.Execute(context.Background(), "inv-1", first); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	_, err := ex.Execute(context.Background(), "inv-1", masterCall("call-1", "create_location", json.RawMessage(`{"name":"Other","parent_id":null,"type":"world"}`)))
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
	call := masterCall("call-1", "create_location", json.RawMessage(`{"name":"X","parent_id":0,"type":"region"}`))

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
