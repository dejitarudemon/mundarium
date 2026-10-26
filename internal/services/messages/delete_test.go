package messages_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
)

func TestDeleteMessages(t *testing.T) {
	resetDB(t)
	_ = seedMessage(t, "inv-1", "first")
	_ = seedMessage(t, "inv-1", "second")
	_ = seedMessage(t, "inv-2", "other")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	if err := testSvc.DeleteMessages(ctx, "inv-1"); err != nil {
		t.Fatalf("DeleteMessages: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Delete || rec.calls[0].Entity != "MESSAGE" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	left, err := testRepo.GetMessages(ctx, "inv-1", nil)
	if err != nil {
		t.Fatalf("GetMessages inv-1: %v", err)
	}
	if len(left) != 0 {
		t.Fatalf("want inv-1 empty, got %+v", left)
	}

	other, err := testRepo.GetMessages(ctx, "inv-2", nil)
	if err != nil {
		t.Fatalf("GetMessages inv-2: %v", err)
	}
	if len(other) != 1 || other[0].Text != "other" {
		t.Fatalf("want inv-2 untouched, got %+v", other)
	}

	// Idempotent: delete again succeeds.
	if err := testSvc.DeleteMessages(ctx, "inv-1"); err != nil {
		t.Fatalf("DeleteMessages again: %v", err)
	}
}

func TestDeleteMessages_EmptyInvokeID(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	err := testSvc.DeleteMessages(ctx, "")
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want errs.InvalidRequest, got %v", err)
	}
	if len(rec.failCalls) == 0 {
		t.Fatal("want Fail called")
	}
}
