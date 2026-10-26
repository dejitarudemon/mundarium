package messages_test

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/alarm"
	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/actions"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"gorm.io/gorm"
)

var (
	_ tools.Tool = (*toolsmessages.CreateMessageTool)(nil)
)

func newExecCtx(t *testing.T) (execution.ExecutionContext, context.CancelFunc) {
	t.Helper()

	complete := func(context.Context, journal.ID, journal.ID, journal.Result, *gorm.DB) error {
		return nil
	}
	fail := func(context.Context, journal.ID, journal.ID, error, *gorm.DB) error {
		return nil
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		journal.ID("test-request"),
		journal.ID("inv-1"),
		providers.Role_Master,
		complete,
		fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func TestNewCreateMessageTool_NilService(t *testing.T) {
	_, err := toolsmessages.NewCreateMessageTool(createMessageDefinitionPath(t), nil, nil)
	if !errors.Is(err, actions.ErrServiceIsNil) {
		t.Fatalf("want ErrServiceIsNil, got %v", err)
	}
}

func TestNewCreateMessageTool_LoadsDefinition(t *testing.T) {
	tool, err := toolsmessages.NewCreateMessageTool(createMessageDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateMessageTool: %v", err)
	}

	def := tool.Definition()
	if def.Name != "send_message" {
		t.Fatalf("want definition name send_message, got %q", def.Name)
	}
	def.Name = "mutated"
	again := tool.Definition()
	if again.Name != "send_message" {
		t.Fatalf("Definition must return a copy, got %q", again.Name)
	}
	if tool.IsIdempotent() {
		t.Fatal("send_message must not be idempotent")
	}
}

func TestCreateMessageTool_Execute(t *testing.T) {
	resetDB(t)

	al := alarm.NewAlarm(1)
	tool, err := toolsmessages.NewCreateMessageTool(createMessageDefinitionPath(t), testSvc, al)
	if err != nil {
		t.Fatalf("NewCreateMessageTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	raw, err := tool.Execute(ctx, json.RawMessage(`{"text":"hello"}`))
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}

	var out struct {
		ID       repositories.ID `json:"id"`
		InvokeID string          `json:"invoke_id"`
		SeqNum   uint            `json:"seq_num"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out.ID == 0 || out.InvokeID != "inv-1" || out.SeqNum != 0 {
		t.Fatalf("unexpected output: %+v", out)
	}

	got, err := testRepo.GetMessage(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if got.Text != "hello" || string(got.From) != "master" {
		t.Fatalf("want text/from from args+context, got text=%q from=%q", got.Text, got.From)
	}

	waitCtx, waitCancel := context.WithTimeout(context.Background(), time.Second)
	defer waitCancel()
	if err := al.Wait(waitCtx); err != nil {
		t.Fatalf("alarm was not rung: %v", err)
	}
}

func TestCreateMessageTool_Execute_InvalidJSON(t *testing.T) {
	tool, err := toolsmessages.NewCreateMessageTool(createMessageDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewCreateMessageTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{`))
	if err == nil {
		t.Fatal("expected invalid json error")
	}
}
