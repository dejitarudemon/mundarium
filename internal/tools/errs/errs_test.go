package errs_test

import (
	"errors"
	"strings"
	"testing"

	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/google/uuid"
)

func TestServiceError_DelegatesToService(t *testing.T) {
	tb := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	svcErr := svcerrs.NewNotFoundWithTracebackID("location", "42", tb)

	got := toolerrs.NewServiceError(svcErr)
	if got.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, got.TracebackID())
	}
	if got.IsRetryable() {
		t.Fatal("NotFound-backed ServiceError must not be retryable")
	}
	if !errors.Is(got, svcErr) {
		t.Fatalf("Unwrap: want %v via Is", svcErr)
	}
	if !strings.Contains(got.Error(), "location") {
		t.Fatalf("message missing service detail: %q", got.Error())
	}

	retryable := toolerrs.NewServiceError(svcerrs.NewInternalError(errors.New("db")))
	if !retryable.IsRetryable() {
		t.Fatal("InternalError-backed ServiceError must be retryable")
	}
}

func TestInvalidRequest(t *testing.T) {
	tb := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	cause := errors.New("unexpected EOF")

	got := toolerrs.NewInvalidRequestWithTracebackID(cause, tb)
	if got.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, got.TracebackID())
	}
	if got.IsRetryable() {
		t.Fatal("InvalidRequest must not be retryable")
	}
	if !errors.Is(got, cause) {
		t.Fatalf("Unwrap: want %v", cause)
	}
	if !strings.Contains(got.Error(), "unexpected EOF") {
		t.Fatalf("message missing cause: %q", got.Error())
	}

	generated := toolerrs.NewInvalidRequest(cause)
	if generated.TracebackID() == uuid.Nil {
		t.Fatal("NewInvalidRequest must set traceback id")
	}
}

func TestInternalError(t *testing.T) {
	tb := uuid.MustParse("44444444-4444-4444-4444-444444444444")
	cause := errors.New("disk full")

	got := toolerrs.NewInternalErrorWithTracebackID(cause, tb)
	if got.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, got.TracebackID())
	}
	if !got.IsRetryable() {
		t.Fatal("InternalError must be retryable")
	}
	if !errors.Is(got, cause) {
		t.Fatalf("Unwrap: want %v", cause)
	}
	if !strings.Contains(got.Error(), "disk full") {
		t.Fatalf("message missing cause: %q", got.Error())
	}

	generated := toolerrs.NewInternalError(cause)
	if generated.TracebackID() == uuid.Nil {
		t.Fatal("NewInternalError must set traceback id")
	}
}
