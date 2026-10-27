// Package errs defines shared tool-layer errors with traceback ids.
package errs

import (
	"fmt"

	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	"github.com/google/uuid"
)

var (
	_ ToolErr = ServiceError{}
	_ ToolErr = InvalidRequest{}
	_ ToolErr = InternalError{}
)

// ToolErr is a tool-domain error that carries a traceback id
// and reports whether the failed operation may be retried.
//
// isToolErr seals the interface so service-layer errors with the same
// method set do not accidentally satisfy ToolErr.
type ToolErr interface {
	error

	TracebackID() uuid.UUID

	// IsRetryable reports whether the caller may attempt the operation again.
	IsRetryable() bool

	isToolErr()
}

func generateTracebackID() uuid.UUID {
	return uuid.New()
}

// ServiceError wraps a service-layer [svcerrs.ServiceError].
// TracebackID and IsRetryable are delegated to the underlying service error.
type ServiceError struct {
	err svcerrs.ServiceError
}

// NewServiceError creates a ServiceError around a non-nil service error.
func NewServiceError(err svcerrs.ServiceError) ServiceError {
	return ServiceError{err: err}
}

func (e ServiceError) Error() string {
	return fmt.Sprintf("tool service error: %v", e.err)
}

// TracebackID returns the traceback id from the underlying service error.
func (e ServiceError) TracebackID() uuid.UUID {
	return e.err.TracebackID()
}

// IsRetryable reports whether the underlying service error may be retried.
func (e ServiceError) IsRetryable() bool {
	return e.err.IsRetryable()
}

// Unwrap returns the underlying service error.
func (e ServiceError) Unwrap() error {
	return e.err
}

func (ServiceError) isToolErr() {}

// InvalidRequest indicates a tool argument serialization or deserialization failure.
type InvalidRequest struct {
	tracebackID uuid.UUID
	err         error
}

// NewInvalidRequest creates an InvalidRequest around a ser/deser error.
func NewInvalidRequest(err error) InvalidRequest {
	return InvalidRequest{
		tracebackID: generateTracebackID(),
		err:         err,
	}
}

// NewInvalidRequestWithTracebackID creates an InvalidRequest with a fixed traceback id.
func NewInvalidRequestWithTracebackID(err error, tracebackID uuid.UUID) InvalidRequest {
	return InvalidRequest{
		tracebackID: tracebackID,
		err:         err,
	}
}

func (e InvalidRequest) Error() string {
	return fmt.Sprintf("tool invalid request: %v", e.err)
}

// TracebackID returns the error traceback identifier.
func (e InvalidRequest) TracebackID() uuid.UUID {
	return e.tracebackID
}

// IsRetryable reports that invalid tool requests should not be retried.
func (e InvalidRequest) IsRetryable() bool {
	return false
}

// Unwrap returns the underlying ser/deser error.
func (e InvalidRequest) Unwrap() error {
	return e.err
}

func (InvalidRequest) isToolErr() {}

// InternalError wraps an unexpected tool infrastructure failure.
type InternalError struct {
	tracebackID uuid.UUID
	err         error
}

// NewInternalError creates an InternalError around err.
func NewInternalError(err error) InternalError {
	return InternalError{
		tracebackID: generateTracebackID(),
		err:         err,
	}
}

// NewInternalErrorWithTracebackID creates an InternalError with a fixed traceback id.
func NewInternalErrorWithTracebackID(err error, tracebackID uuid.UUID) InternalError {
	return InternalError{
		tracebackID: tracebackID,
		err:         err,
	}
}

func (e InternalError) Error() string {
	return fmt.Sprintf("tool internal error: %v", e.err)
}

// TracebackID returns the error traceback identifier.
func (e InternalError) TracebackID() uuid.UUID {
	return e.tracebackID
}

// Unwrap returns the underlying error.
func (e InternalError) Unwrap() error {
	return e.err
}

// IsRetryable reports that transient internal failures may be retried.
func (e InternalError) IsRetryable() bool {
	return true
}

func (InternalError) isToolErr() {}
