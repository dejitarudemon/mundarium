// Package errs defines shared service-layer errors with traceback ids.
//
// Concrete services may return these types; this package does not depend on them.
package errs

import (
	"fmt"

	"github.com/google/uuid"
)

var (
	_ ServiceError = AlreadyCreated{}
	_ ServiceError = NotFound{}
	_ ServiceError = InvalidRequest{}
	_ ServiceError = InternalError{}
)

// ServiceError is a service-domain error that carries a traceback id
// and reports whether the failed operation may be retried.
type ServiceError interface {
	error

	TracebackID() uuid.UUID

	// IsRetryable reports whether the caller may attempt the operation again.
	IsRetryable() bool
}

func generateTracebackID() uuid.UUID {
	return uuid.New()
}

// AlreadyCreated indicates a resource already exists and cannot be created again.
type AlreadyCreated struct {
	tracebackID uuid.UUID
	Entity      string
	ID          string
}

// NewAlreadyCreated creates an AlreadyCreated error for entity/id.
func NewAlreadyCreated(entity, id string) AlreadyCreated {
	return AlreadyCreated{
		tracebackID: generateTracebackID(),
		Entity:      entity,
		ID:          id,
	}
}

// NewAlreadyCreatedWithTracebackID creates an AlreadyCreated error with a fixed traceback id.
func NewAlreadyCreatedWithTracebackID(entity, id string, tracebackID uuid.UUID) AlreadyCreated {
	return AlreadyCreated{
		tracebackID: tracebackID,
		Entity:      entity,
		ID:          id,
	}
}

func (e AlreadyCreated) Error() string {
	return fmt.Sprintf("%s (%s) is already created", e.Entity, e.ID)
}

// TracebackID returns the error traceback identifier.
func (e AlreadyCreated) TracebackID() uuid.UUID {
	return e.tracebackID
}

// IsRetryable reports that an already-created resource should not be retried.
func (e AlreadyCreated) IsRetryable() bool {
	return false
}

// NotFound indicates a requested resource is missing.
type NotFound struct {
	tracebackID uuid.UUID
	Entity      string
	ID          string
}

// NewNotFound creates a NotFound error for entity/id.
func NewNotFound(entity, id string) NotFound {
	return NotFound{
		tracebackID: generateTracebackID(),
		Entity:      entity,
		ID:          id,
	}
}

// NewNotFoundWithTracebackID creates a NotFound error with a fixed traceback id.
func NewNotFoundWithTracebackID(entity, id string, tracebackID uuid.UUID) NotFound {
	return NotFound{
		tracebackID: tracebackID,
		Entity:      entity,
		ID:          id,
	}
}

func (e NotFound) Error() string {
	return fmt.Sprintf("%s (%s) not found", e.Entity, e.ID)
}

// TracebackID returns the error traceback identifier.
func (e NotFound) TracebackID() uuid.UUID {
	return e.tracebackID
}

// IsRetryable reports that a missing resource should not be retried.
func (e NotFound) IsRetryable() bool {
	return false
}

// InvalidRequest indicates the caller supplied an invalid service request.
type InvalidRequest struct {
	tracebackID uuid.UUID
	Reason      string
}

// NewInvalidRequest creates an InvalidRequest error with reason.
func NewInvalidRequest(reason string) InvalidRequest {
	return InvalidRequest{
		tracebackID: generateTracebackID(),
		Reason:      reason,
	}
}

// NewInvalidRequestWithTracebackID creates an InvalidRequest error with a fixed traceback id.
func NewInvalidRequestWithTracebackID(reason string, tracebackID uuid.UUID) InvalidRequest {
	return InvalidRequest{
		tracebackID: tracebackID,
		Reason:      reason,
	}
}

func (e InvalidRequest) Error() string {
	return fmt.Sprintf("invalid request: %s", e.Reason)
}

// TracebackID returns the error traceback identifier.
func (e InvalidRequest) TracebackID() uuid.UUID {
	return e.tracebackID
}

// IsRetryable reports that invalid requests should not be retried.
func (e InvalidRequest) IsRetryable() bool {
	return false
}

// InternalError wraps an unexpected service infrastructure failure.
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
	return fmt.Sprintf("service internal error: %v", e.err)
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
