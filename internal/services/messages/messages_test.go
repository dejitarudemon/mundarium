package messages_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
)

func TestCreateMessage(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.CreateMessage(ctx, svcmessages.CreateMessageInput{
		InvokeID: "inv-1",
		Text:     "hello",
		From:     "master",
	})
	if err != nil {
		t.Fatalf("CreateMessage: %v", err)
	}
	if out.ID == 0 || out.InvokeID != "inv-1" || out.SeqNum != 0 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Create || rec.calls[0].Entity != "MESSAGE" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	second, err := testSvc.CreateMessage(ctx, svcmessages.CreateMessageInput{
		InvokeID: "inv-1",
		Text:     "again",
		From:     "master",
	})
	if err != nil {
		t.Fatalf("CreateMessage second: %v", err)
	}
	if second.SeqNum != 1 || second.ID == out.ID {
		t.Fatalf("create is not idempotent; want new row seq_num=1, got %+v", second)
	}
}

func TestCreateMessage_ValidationErrors(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	cases := []struct {
		name string
		in   svcmessages.CreateMessageInput
	}{
		{name: "empty invoke", in: svcmessages.CreateMessageInput{Text: "x", From: "master"}},
		{name: "empty text", in: svcmessages.CreateMessageInput{InvokeID: "inv", From: "master"}},
		{name: "empty from", in: svcmessages.CreateMessageInput{InvokeID: "inv", Text: "x"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := len(rec.failCalls)
			_, err := testSvc.CreateMessage(ctx, tc.in)
			if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
				t.Fatalf("want errs.InvalidRequest, got %v", err)
			}
			if len(rec.failCalls) <= before {
				t.Fatal("want Fail called on validation error")
			}
		})
	}
}

func TestCreateMessage_CompleteError(t *testing.T) {
	resetDB(t)
	cause := errors.New("callback failed")
	rec := &completeRecorder{err: cause}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	_, err := testSvc.CreateMessage(ctx, svcmessages.CreateMessageInput{
		InvokeID: "inv-1",
		Text:     "x",
		From:     "master",
	})
	if _, ok := errors.AsType[svcerrs.InternalError](err); !ok || !errors.Is(err, cause) {
		t.Fatalf("want errs.InternalError wrapping callback failed, got %v", err)
	}

	var count int64
	if err := testConn.DB().Table("messages").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("create should roll back when Complete fails, count=%d", count)
	}
}

func TestGetMessage(t *testing.T) {
	resetDB(t)
	msg := seedMessage(t, "inv-1", "hello")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.GetMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if out.ID != msg.ID || out.InvokeID != "inv-1" || out.Text != "hello" || out.From != "master" || out.SeqNum != 0 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Get {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestGetMessage_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.GetMessage(ctx, 999)
	if _, ok := errors.AsType[svcerrs.NotFound](err); !ok {
		t.Fatalf("expected errs.NotFound, got %v", err)
	}
}

func TestGetMessages(t *testing.T) {
	resetDB(t)
	_ = seedMessage(t, "inv-1", "first")
	_ = seedMessage(t, "inv-1", "second")
	_ = seedMessage(t, "inv-2", "other")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.GetMessages(ctx, "inv-1", nil)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(out) != 2 || out[0].Text != "first" || out[0].SeqNum != 0 || out[1].Text != "second" || out[1].SeqNum != 1 {
		t.Fatalf("unexpected output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Get {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestGetMessages_StartFrom(t *testing.T) {
	resetDB(t)
	_ = seedMessage(t, "inv-1", "first")
	_ = seedMessage(t, "inv-1", "second")
	_ = seedMessage(t, "inv-1", "third")

	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	startFrom := uint(1)
	out, err := testSvc.GetMessages(ctx, "inv-1", &startFrom)
	if err != nil {
		t.Fatalf("GetMessages: %v", err)
	}
	if len(out) != 2 || out[0].Text != "second" || out[0].SeqNum != 1 || out[1].Text != "third" || out[1].SeqNum != 2 {
		t.Fatalf("unexpected output: %+v", out)
	}
}

func TestGetMessages_EmptyInvokeID(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	_, err := testSvc.GetMessages(ctx, "", nil)
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want errs.InvalidRequest, got %v", err)
	}
	if len(rec.failCalls) == 0 {
		t.Fatal("want Fail called")
	}
}
