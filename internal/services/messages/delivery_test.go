package messages_test

import (
	"errors"
	"testing"
	"time"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"gorm.io/gorm"
)

func TestSetDeliveredAt(t *testing.T) {
	resetDB(t)
	msg := seedMessage(t, "inv-1", "hello")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	at := time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	if err := testSvc.SetDeliveredAt(ctx, msg.ID, &at); err != nil {
		t.Fatalf("SetDeliveredAt: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != libjournal.Action_Patch || rec.calls[0].Entity != "MESSAGE" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	got, err := testRepo.GetMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if got.DeliveredAt == nil || !got.DeliveredAt.Equal(at) {
		t.Fatalf("want delivered_at %v, got %v", at, got.DeliveredAt)
	}

	// Idempotent: same timestamp again succeeds and keeps the value.
	if err := testSvc.SetDeliveredAt(ctx, msg.ID, &at); err != nil {
		t.Fatalf("SetDeliveredAt again: %v", err)
	}
	got, err = testRepo.GetMessage(ctx, msg.ID)
	if err != nil {
		t.Fatalf("GetMessage after repeat: %v", err)
	}
	if got.DeliveredAt == nil || !got.DeliveredAt.Equal(at) {
		t.Fatalf("want delivered_at unchanged %v, got %v", at, got.DeliveredAt)
	}
}

func TestSetDeliveredAt_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	at := time.Now().UTC()
	err := testSvc.SetDeliveredAt(ctx, 999, &at)
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected ErrRecordNotFound, got %v", err)
	}
}
