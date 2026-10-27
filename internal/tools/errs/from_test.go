package errs_test

import (
	"errors"
	"testing"

	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/google/uuid"
)

func TestFrom_Nil(t *testing.T) {
	if got := toolerrs.From(nil); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestFrom_AlreadyToolErr(t *testing.T) {
	src := toolerrs.NewInvalidRequest(errors.New("bad json"))
	got := toolerrs.From(src)
	if got != src {
		t.Fatalf("want same ToolErr, got %T %v", got, got)
	}
}

func TestFrom_ServiceError(t *testing.T) {
	tb := uuid.MustParse("33333333-3333-3333-3333-333333333333")
	svcErr := svcerrs.NewNotFoundWithTracebackID("location", "1", tb)

	got := toolerrs.From(svcErr)
	se, ok := errors.AsType[toolerrs.ServiceError](got)
	if !ok {
		t.Fatalf("want ServiceError, got %T", got)
	}
	if se.TracebackID() != tb {
		t.Fatalf("traceback: want %s, got %s", tb, se.TracebackID())
	}
	if !errors.Is(got, svcErr) {
		t.Fatalf("want unwrap to service error")
	}
}

func TestFrom_Unknown(t *testing.T) {
	cause := errors.New("boom")
	got := toolerrs.From(cause)
	ie, ok := errors.AsType[toolerrs.InternalError](got)
	if !ok {
		t.Fatalf("want InternalError, got %T", got)
	}
	if !errors.Is(ie, cause) {
		t.Fatalf("want unwrap to cause")
	}
	if !ie.IsRetryable() {
		t.Fatal("InternalError must be retryable")
	}
}
