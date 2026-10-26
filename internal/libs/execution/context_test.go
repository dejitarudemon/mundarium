package execution_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"gorm.io/gorm"
)

func TestNewExecutionContext_NilCallback(t *testing.T) {
	_, _, err := execution.NewExecutionContext(
		context.Background(),
		"req",
		"grp",
		providers.Role_Master,
		nil,
		func(context.Context, libjournal.ID, libjournal.ID, error, *gorm.DB) error { return nil },
	)
	if err == nil || err.Error() != "nil callback" {
		t.Fatalf("want nil callback, got %v", err)
	}
}

func TestExecutionContext_CompleteAndFailForwardIDsAndTx(t *testing.T) {
	var gotReq, gotGroup libjournal.ID
	var gotTX *gorm.DB
	var gotResult libjournal.Result
	var gotCause error

	complete := func(ctx context.Context, requestID, groupRequestID libjournal.ID, result libjournal.Result, tx *gorm.DB) error {
		gotReq, gotGroup, gotResult, gotTX = requestID, groupRequestID, result, tx
		return nil
	}
	fail := func(ctx context.Context, requestID, groupRequestID libjournal.ID, cause error, tx *gorm.DB) error {
		gotReq, gotGroup, gotCause, gotTX = requestID, groupRequestID, cause, tx
		return nil
	}

	tx := &gorm.DB{}
	ctx, cancel, err := execution.NewExecutionContext(context.Background(), "call-1", "inv-1", providers.Role_Master, complete, fail)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	defer cancel()

	if ctx.InvokeID() != "inv-1" || ctx.Role() != providers.Role_Master {
		t.Fatalf("want invoke inv-1 role master, got %s %s", ctx.InvokeID(), ctx.Role())
	}

	result := libjournal.Result{Entity: "LOCATION"}
	if err := ctx.Complete(result, tx); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if gotReq != "call-1" || gotGroup != "inv-1" || gotResult.Entity != "LOCATION" || gotTX != tx {
		t.Fatalf("Complete forward mismatch: req=%s group=%s result=%#v tx=%v", gotReq, gotGroup, gotResult, gotTX)
	}

	cause := errors.New("boom")
	if err := ctx.Fail(cause, tx); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if gotReq != "call-1" || gotGroup != "inv-1" || gotCause != cause || gotTX != tx {
		t.Fatalf("Fail forward mismatch: req=%s group=%s cause=%v tx=%v", gotReq, gotGroup, gotCause, gotTX)
	}
}

func TestExecutionContext_WithoutCallbackNoops(t *testing.T) {
	ctx, cancel := execution.NewExecutionContextWithoutCallback(context.Background(), "call-1", "inv-1", providers.Role_Master)
	defer cancel()

	if err := ctx.Complete(libjournal.Result{}, nil); err != nil {
		t.Fatalf("Complete noop: %v", err)
	}
	if err := ctx.Fail(errors.New("x"), nil); err != nil {
		t.Fatalf("Fail noop: %v", err)
	}
}
