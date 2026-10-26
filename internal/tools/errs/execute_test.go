package errs_test

import (
	"errors"
	"testing"

	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/google/uuid"
)

func TestExecuteErrors_ToolErr(t *testing.T) {
	tb := uuid.MustParse("55555555-5555-5555-5555-555555555555")

	cases := []struct {
		name      string
		err       toolerrs.ToolErr
		retryable bool
	}{
		{"ToolNotFound", toolerrs.NewToolNotFoundWithTracebackID(tb), false},
		{"PermissionDenied", toolerrs.NewPermissionDeniedWithTracebackID(tb), false},
		{"RequestToolMismatch", toolerrs.NewRequestToolMismatchWithTracebackID(tb), false},
		{"RequestArgsMismatch", toolerrs.NewRequestArgsMismatchWithTracebackID(tb), false},
		{"RequestStillRunning", toolerrs.NewRequestStillRunningWithTracebackID(tb), true},
		{"RequestAlreadyFailed", toolerrs.NewRequestAlreadyFailedWithTracebackID(tb), false},
		{"UnexpectedRequestStatus", toolerrs.NewUnexpectedRequestStatusWithTracebackID(tb), true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.TracebackID() != tb {
				t.Fatalf("traceback: want %s, got %s", tb, tc.err.TracebackID())
			}
			if got := tc.err.IsRetryable(); got != tc.retryable {
				t.Fatalf("IsRetryable: want %v, got %v", tc.retryable, got)
			}
			if tc.err.Error() == "" {
				t.Fatal("empty Error()")
			}
			if !errors.Is(tc.err, tc.err) {
				t.Fatal("errors.Is self must hold via Is method")
			}
		})
	}
}
