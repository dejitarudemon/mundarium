package errs

import (
	"github.com/google/uuid"
)

var (
	_ ToolErr = ToolNotFound{}
	_ ToolErr = PermissionDenied{}
	_ ToolErr = RequestToolMismatch{}
	_ ToolErr = RequestArgsMismatch{}
	_ ToolErr = RequestStillRunning{}
	_ ToolErr = RequestAlreadyFailed{}
	_ ToolErr = UnexpectedRequestStatus{}
)

// ToolNotFound indicates the requested tool name is not registered.
type ToolNotFound struct {
	tracebackID uuid.UUID
}

// NewToolNotFound creates a ToolNotFound error.
func NewToolNotFound() ToolNotFound {
	return ToolNotFound{tracebackID: generateTracebackID()}
}

// NewToolNotFoundWithTracebackID creates a ToolNotFound with a fixed traceback id.
func NewToolNotFoundWithTracebackID(tracebackID uuid.UUID) ToolNotFound {
	return ToolNotFound{tracebackID: tracebackID}
}

func (e ToolNotFound) Error() string { return "tool not found" }

// TracebackID returns the error traceback identifier.
func (e ToolNotFound) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that a missing tool should not be retried.
func (e ToolNotFound) IsRetryable() bool { return false }

func (e ToolNotFound) Is(target error) bool {
	_, ok := target.(ToolNotFound)
	return ok
}

func (ToolNotFound) isToolErr() {}

// PermissionDenied indicates the caller's role may not invoke the tool.
type PermissionDenied struct {
	tracebackID uuid.UUID
}

// NewPermissionDenied creates a PermissionDenied error.
func NewPermissionDenied() PermissionDenied {
	return PermissionDenied{tracebackID: generateTracebackID()}
}

// NewPermissionDeniedWithTracebackID creates a PermissionDenied with a fixed traceback id.
func NewPermissionDeniedWithTracebackID(tracebackID uuid.UUID) PermissionDenied {
	return PermissionDenied{tracebackID: tracebackID}
}

func (e PermissionDenied) Error() string { return "tool call denied" }

// TracebackID returns the error traceback identifier.
func (e PermissionDenied) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that a permission denial should not be retried.
func (e PermissionDenied) IsRetryable() bool { return false }

func (e PermissionDenied) Is(target error) bool {
	_, ok := target.(PermissionDenied)
	return ok
}

func (PermissionDenied) isToolErr() {}

// RequestToolMismatch is returned when a request id is already registered under another tool name.
type RequestToolMismatch struct {
	tracebackID uuid.UUID
}

// NewRequestToolMismatch creates a RequestToolMismatch error.
func NewRequestToolMismatch() RequestToolMismatch {
	return RequestToolMismatch{tracebackID: generateTracebackID()}
}

// NewRequestToolMismatchWithTracebackID creates a RequestToolMismatch with a fixed traceback id.
func NewRequestToolMismatchWithTracebackID(tracebackID uuid.UUID) RequestToolMismatch {
	return RequestToolMismatch{tracebackID: tracebackID}
}

func (e RequestToolMismatch) Error() string {
	return "request already registered for a different tool"
}

// TracebackID returns the error traceback identifier.
func (e RequestToolMismatch) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that a tool mismatch should not be retried.
func (e RequestToolMismatch) IsRetryable() bool { return false }

func (e RequestToolMismatch) Is(target error) bool {
	_, ok := target.(RequestToolMismatch)
	return ok
}

func (RequestToolMismatch) isToolErr() {}

// RequestArgsMismatch is returned when a request id is reused with different tool args.
type RequestArgsMismatch struct {
	tracebackID uuid.UUID
}

// NewRequestArgsMismatch creates a RequestArgsMismatch error.
func NewRequestArgsMismatch() RequestArgsMismatch {
	return RequestArgsMismatch{tracebackID: generateTracebackID()}
}

// NewRequestArgsMismatchWithTracebackID creates a RequestArgsMismatch with a fixed traceback id.
func NewRequestArgsMismatchWithTracebackID(tracebackID uuid.UUID) RequestArgsMismatch {
	return RequestArgsMismatch{tracebackID: tracebackID}
}

func (e RequestArgsMismatch) Error() string {
	return "request already registered with different args"
}

// TracebackID returns the error traceback identifier.
func (e RequestArgsMismatch) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that an args mismatch should not be retried.
func (e RequestArgsMismatch) IsRetryable() bool { return false }

func (e RequestArgsMismatch) Is(target error) bool {
	_, ok := target.(RequestArgsMismatch)
	return ok
}

func (RequestArgsMismatch) isToolErr() {}

// RequestStillRunning is returned when a non-idempotent request is still in progress.
type RequestStillRunning struct {
	tracebackID uuid.UUID
}

// NewRequestStillRunning creates a RequestStillRunning error.
func NewRequestStillRunning() RequestStillRunning {
	return RequestStillRunning{tracebackID: generateTracebackID()}
}

// NewRequestStillRunningWithTracebackID creates a RequestStillRunning with a fixed traceback id.
func NewRequestStillRunningWithTracebackID(tracebackID uuid.UUID) RequestStillRunning {
	return RequestStillRunning{tracebackID: tracebackID}
}

func (e RequestStillRunning) Error() string { return "request is still running" }

// TracebackID returns the error traceback identifier.
func (e RequestStillRunning) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that a still-running request may be polled again later.
func (e RequestStillRunning) IsRetryable() bool { return true }

func (e RequestStillRunning) Is(target error) bool {
	_, ok := target.(RequestStillRunning)
	return ok
}

func (RequestStillRunning) isToolErr() {}

// RequestAlreadyFailed is returned when a non-idempotent request previously failed without a cause.
type RequestAlreadyFailed struct {
	tracebackID uuid.UUID
}

// NewRequestAlreadyFailed creates a RequestAlreadyFailed error.
func NewRequestAlreadyFailed() RequestAlreadyFailed {
	return RequestAlreadyFailed{tracebackID: generateTracebackID()}
}

// NewRequestAlreadyFailedWithTracebackID creates a RequestAlreadyFailed with a fixed traceback id.
func NewRequestAlreadyFailedWithTracebackID(tracebackID uuid.UUID) RequestAlreadyFailed {
	return RequestAlreadyFailed{tracebackID: tracebackID}
}

func (e RequestAlreadyFailed) Error() string { return "request already failed" }

// TracebackID returns the error traceback identifier.
func (e RequestAlreadyFailed) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that an already-failed request should not be retried.
func (e RequestAlreadyFailed) IsRetryable() bool { return false }

func (e RequestAlreadyFailed) Is(target error) bool {
	_, ok := target.(RequestAlreadyFailed)
	return ok
}

func (RequestAlreadyFailed) isToolErr() {}

// UnexpectedRequestStatus is returned when a journaled request has an unknown status.
type UnexpectedRequestStatus struct {
	tracebackID uuid.UUID
}

// NewUnexpectedRequestStatus creates an UnexpectedRequestStatus error.
func NewUnexpectedRequestStatus() UnexpectedRequestStatus {
	return UnexpectedRequestStatus{tracebackID: generateTracebackID()}
}

// NewUnexpectedRequestStatusWithTracebackID creates an UnexpectedRequestStatus with a fixed traceback id.
func NewUnexpectedRequestStatusWithTracebackID(tracebackID uuid.UUID) UnexpectedRequestStatus {
	return UnexpectedRequestStatus{tracebackID: tracebackID}
}

func (e UnexpectedRequestStatus) Error() string { return "unexpected request status" }

// TracebackID returns the error traceback identifier.
func (e UnexpectedRequestStatus) TracebackID() uuid.UUID { return e.tracebackID }

// IsRetryable reports that an unexpected journal status may be retried.
func (e UnexpectedRequestStatus) IsRetryable() bool { return true }

func (e UnexpectedRequestStatus) Is(target error) bool {
	_, ok := target.(UnexpectedRequestStatus)
	return ok
}

func (UnexpectedRequestStatus) isToolErr() {}
