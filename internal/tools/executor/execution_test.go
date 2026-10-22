package executor_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/journal/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	libtools "github.com/dejitarudemon/mundarium/internal/libs/tools"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
)

type stubTool struct {
	name        string
	idempotent  bool
	executions  atomic.Int32
	executeErr  error
	executeBody json.RawMessage
	complete    bool
}

func (t *stubTool) Definition() definition.Definition {
	return definition.Definition{Name: t.name}
}

func (t *stubTool) IsIdempotent() bool { return t.idempotent }

func (t *stubTool) Execute(ctx execution.ExecutionContext, args json.RawMessage) (json.RawMessage, error) {
	t.executions.Add(1)
	if t.executeErr != nil {
		_ = ctx.Fail(t.executeErr)
		return nil, t.executeErr
	}
	if t.complete {
		if err := ctx.Complete(journal.Result{
			Action: libjournal.Action_Create,
			Entity: "STUB",
			Data:   map[string]any{"ok": true, "args": string(args)},
		}); err != nil {
			return nil, err
		}
	}
	if len(t.executeBody) > 0 {
		return t.executeBody, nil
	}
	return json.RawMessage(`{"ok":true}`), nil
}

var _ tools.Tool = (*stubTool)(nil)

func newExecutor(t *testing.T, tool tools.Tool) *executor.Executor {
	t.Helper()

	ex, err := executor.NewExecutor(implemented.NewMapJournal())
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
	ex := newExecutor(t, nil)
	_, err := ex.Execute(context.Background(), "inv-1", libtools.Call{ID: "c1", Name: "missing"})
	if !errors.Is(err, executor.ErrToolNotFound) {
		t.Fatalf("want ErrToolNotFound, got %v", err)
	}
}

func TestExecuteNotIdempotent_RegistersAndRuns(t *testing.T) {
	tool := &stubTool{name: "create_location", complete: true, executeBody: json.RawMessage(`{"id":1}`)}
	ex := newExecutor(t, tool)

	got, err := ex.Execute(context.Background(), "inv-1", libtools.Call{
		ID:   "call-1",
		Name: "create_location",
		Args: json.RawMessage(`{"name":"X"}`),
	})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if string(got) != `{"id":1}` {
		t.Fatalf("unexpected result: %s", got)
	}
	if tool.executions.Load() != 1 {
		t.Fatalf("want 1 execution, got %d", tool.executions.Load())
	}
}

func TestExecuteNotIdempotent_ReplayCompleted(t *testing.T) {
	tool := &stubTool{name: "create_location", complete: true, executeBody: json.RawMessage(`{"id":1}`)}
	ex := newExecutor(t, tool)
	call := libtools.Call{ID: "call-1", Name: "create_location", Args: json.RawMessage(`{}`)}

	if _, err := ex.Execute(context.Background(), "inv-1", call); err != nil {
		t.Fatalf("first Execute: %v", err)
	}

	got, err := ex.Execute(context.Background(), "inv-1", call)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if tool.executions.Load() != 1 {
		t.Fatalf("completed request must not re-execute, got %d", tool.executions.Load())
	}

	var payload map[string]any
	if err := json.Unmarshal(got, &payload); err != nil {
		t.Fatalf("unmarshal replay: %v", err)
	}
	if payload["ok"] != true {
		t.Fatalf("want replayed complete payload, got %#v", payload)
	}
}

func TestExecuteNotIdempotent_StillRunning(t *testing.T) {
	j := implemented.NewMapJournal()
	ex, err := executor.NewExecutor(j)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	tool := &stubTool{name: "create_location", complete: true}
	if err := ex.Register(tool); err != nil {
		t.Fatalf("Register: %v", err)
	}

	requestID := libjournal.RequestID("inv-1:call-1")
	if _, _, err := j.Register(context.Background(), requestID, "create_location"); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err = ex.Execute(context.Background(), "inv-1", libtools.Call{ID: "call-1", Name: "create_location"})
	if !errors.Is(err, executor.ErrRequestStillRunning) {
		t.Fatalf("want ErrRequestStillRunning, got %v", err)
	}
	if tool.executions.Load() != 0 {
		t.Fatalf("running request must not execute again")
	}
}

func TestExecuteNotIdempotent_ToolMismatch(t *testing.T) {
	j := implemented.NewMapJournal()
	ex, err := executor.NewExecutor(j)
	if err != nil {
		t.Fatalf("NewExecutor: %v", err)
	}
	tool := &stubTool{name: "create_location", complete: true}
	if err := ex.Register(tool); err != nil {
		t.Fatalf("Register: %v", err)
	}

	requestID := libjournal.RequestID("inv-1:call-1")
	if _, _, err := j.Register(context.Background(), requestID, "other_tool"); err != nil {
		t.Fatalf("seed Register: %v", err)
	}

	_, err = ex.Execute(context.Background(), "inv-1", libtools.Call{ID: "call-1", Name: "create_location"})
	if !errors.Is(err, executor.ErrRequestToolMismatch) {
		t.Fatalf("want ErrRequestToolMismatch, got %v", err)
	}
}

func TestExecuteNotIdempotent_AlreadyFailed(t *testing.T) {
	tool := &stubTool{name: "create_location", executeErr: errors.New("boom")}
	ex := newExecutor(t, tool)
	call := libtools.Call{ID: "call-1", Name: "create_location", Args: json.RawMessage(`{}`)}

	_, err := ex.Execute(context.Background(), "inv-1", call)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want boom, got %v", err)
	}

	_, err = ex.Execute(context.Background(), "inv-1", call)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("want replayed boom, got %v", err)
	}
	if tool.executions.Load() != 1 {
		t.Fatalf("failed request must not re-execute, got %d", tool.executions.Load())
	}
}
