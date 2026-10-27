package errs_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/services/errs"
	"github.com/google/uuid"
)

func TestServiceErrors_TracebackAndMessages(t *testing.T) {
	tb := uuid.MustParse("11111111-1111-1111-1111-111111111111")

	already := errs.NewAlreadyCreatedWithTracebackID("location", "42", tb)
	if !strings.Contains(already.Error(), "location") || !strings.Contains(already.Error(), "42") {
		t.Fatalf("AlreadyCreated message missing fields: %q", already.Error())
	}
	if already.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, already.TracebackID())
	}
	if errs.NewAlreadyCreated("location", "42").TracebackID() == uuid.Nil {
		t.Fatal("NewAlreadyCreated must set traceback id")
	}
	if already.IsRetryable() {
		t.Fatal("AlreadyCreated must not be retryable")
	}

	notFound := errs.NewNotFoundWithTracebackID("message", "7", tb)
	if !strings.Contains(notFound.Error(), "message") || !strings.Contains(notFound.Error(), "7") {
		t.Fatalf("NotFound message missing fields: %q", notFound.Error())
	}
	if errs.NewNotFound("message", "7").TracebackID() == uuid.Nil {
		t.Fatal("NewNotFound must set traceback id")
	}
	if notFound.IsRetryable() {
		t.Fatal("NotFound must not be retryable")
	}

	invalid := errs.NewInvalidRequestWithTracebackID("invoke id is empty", tb)
	if !strings.Contains(invalid.Error(), "invoke id is empty") {
		t.Fatalf("InvalidRequest message missing reason: %q", invalid.Error())
	}
	if invalid.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, invalid.TracebackID())
	}
	if errs.NewInvalidRequest("x").TracebackID() == uuid.Nil {
		t.Fatal("NewInvalidRequest must set traceback id")
	}
	if invalid.IsRetryable() {
		t.Fatal("InvalidRequest must not be retryable")
	}

	cause := errors.New("db down")
	internal := errs.NewInternalErrorWithTracebackID(cause, tb)
	if !strings.Contains(internal.Error(), "db down") {
		t.Fatalf("InternalError message missing cause: %q", internal.Error())
	}
	if internal.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, internal.TracebackID())
	}
	if !errors.Is(internal, cause) {
		t.Fatalf("Unwrap: want %v, got %v", cause, internal.Unwrap())
	}
	if errs.NewInternalError(cause).TracebackID() == uuid.Nil {
		t.Fatal("NewInternalError must set traceback id")
	}
	if !internal.IsRetryable() {
		t.Fatal("InternalError must be retryable")
	}
}
