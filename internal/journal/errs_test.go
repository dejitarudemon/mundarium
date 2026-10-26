package journal_test

import (
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/google/uuid"
)

func TestJournalErrors_IncludeGroup(t *testing.T) {
	reqID := libjournal.ID("call-1")
	groupID := libjournal.ID("inv-1")
	tb := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	already := journal.NewAlreadyRegisteredWithTracebackID(libjournal.Request{
		ID:      reqID,
		GroupID: groupID,
	}, tb)
	if !strings.Contains(already.Error(), string(reqID)) || !strings.Contains(already.Error(), string(groupID)) {
		t.Fatalf("AlreadyRegistered message missing ids: %q", already.Error())
	}
	if already.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, already.TracebackID())
	}

	generated := journal.NewAlreadyRegistered(libjournal.Request{ID: reqID, GroupID: groupID})
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

	conflict := journal.NewStatusConflictWithTracebackID(reqID, groupID, libjournal.Status_Completed, libjournal.Status_Failed, tb)
	if !strings.Contains(conflict.Error(), string(reqID)) || !strings.Contains(conflict.Error(), string(groupID)) {
		t.Fatalf("StatusConflict message missing ids: %q", conflict.Error())
	}
	if conflict.Got != libjournal.Status_Completed || conflict.Actual != libjournal.Status_Failed {
		t.Fatalf("unexpected conflict statuses: %+v", conflict)
	}
	if journal.NewStatusConflict(reqID, groupID, libjournal.Status_Failed, libjournal.Status_Running).TracebackID() == uuid.Nil {
		t.Fatal("NewStatusConflict must set traceback id")
	}
}
