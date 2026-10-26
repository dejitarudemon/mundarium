package journal_test

import (
	"errors"
	"strings"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/google/uuid"
)

func TestJournalErrors_IncludeGroup(t *testing.T) {
	reqID := journal.ID("call-1")
	groupID := journal.ID("inv-1")
	tb := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	already := journal.NewAlreadyRegisteredWithTracebackID(journal.Request{
		ID:      reqID,
		GroupID: groupID,
	}, tb)
	if !strings.Contains(already.Error(), string(reqID)) || !strings.Contains(already.Error(), string(groupID)) {
		t.Fatalf("AlreadyRegistered message missing ids: %q", already.Error())
	}
	if already.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, already.TracebackID())
	}

	generated := journal.NewAlreadyRegistered(journal.Request{ID: reqID, GroupID: groupID})
	if generated.TracebackID() == uuid.Nil {
		t.Fatal("NewAlreadyRegistered must set traceback id")
	}

	notFound := journal.NewNotFoundWithTracebackID(reqID, groupID, tb)
	if !strings.Contains(notFound.Error(), string(reqID)) || !strings.Contains(notFound.Error(), string(groupID)) {
		t.Fatalf("NotFound message missing ids: %q", notFound.Error())
	}
	if journal.NewNotFound(reqID, groupID).TracebackID() == uuid.Nil {
		t.Fatal("NewNotFound must set traceback id")
	}

	conflict := journal.NewStatusConflictWithTracebackID(reqID, groupID, journal.Status_Completed, journal.Status_Failed, tb)
	if !strings.Contains(conflict.Error(), string(reqID)) || !strings.Contains(conflict.Error(), string(groupID)) {
		t.Fatalf("StatusConflict message missing ids: %q", conflict.Error())
	}
	if conflict.Got != journal.Status_Completed || conflict.Actual != journal.Status_Failed {
		t.Fatalf("unexpected conflict statuses: %+v", conflict)
	}
	if journal.NewStatusConflict(reqID, groupID, journal.Status_Failed, journal.Status_Running).TracebackID() == uuid.Nil {
		t.Fatal("NewStatusConflict must set traceback id")
	}
}

func TestErrorText_RoundTrip(t *testing.T) {
	if got := journal.ErrorTextFrom(nil); got != "" || got.Err() != nil {
		t.Fatalf("nil: want empty, got %q err=%v", got, got.Err())
	}

	src := journal.ErrorTextFrom(errors.New("boom"))
	if src.Error() != "boom" {
		t.Fatalf("Error: want boom, got %q", src.Error())
	}

	val, err := src.Value()
	if err != nil || val != "boom" {
		t.Fatalf("Value: want boom, got %v %v", val, err)
	}

	var loaded journal.ErrorText
	if err := loaded.Scan([]byte("boom")); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if loaded.Err() == nil || loaded.Error() != "boom" {
		t.Fatalf("Scan result: %q", loaded)
	}

	var empty journal.ErrorText
	if err := empty.Scan(nil); err != nil || empty.Err() != nil {
		t.Fatalf("Scan nil: got %q err=%v", empty, err)
	}
}
