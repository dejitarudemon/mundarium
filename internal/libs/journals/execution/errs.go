package journal

import (
	"database/sql/driver"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

var (
	_ JournalError = AlreadyRegistered{}
	_ JournalError = NotFound{}
	_ JournalError = StatusConflict{}
	_ JournalError = InternalErr{}
)

var (
	// ErrGroupIsClosed is returned when registering into a group that was already closed.
	ErrGroupIsClosed = errors.New("group is closed")
	ErrAlreadyFailed = errors.New("request is already failed")
)

// ErrorText is a persisted error message.
// Arbitrary Go error types cannot round-trip through the database; only the message is kept.
type ErrorText string

// ErrorTextFrom stores err's message, or empty if err is nil.
func ErrorTextFrom(err error) ErrorText {
	if err == nil {
		return ""
	}
	return ErrorText(err.Error())
}

// Err returns a non-nil error when the message is non-empty.
func (e ErrorText) Err() error {
	if e == "" {
		return nil
	}
	return e
}

// Error implements the error interface.
func (e ErrorText) Error() string {
	return string(e)
}

// Value implements driver.Valuer for GORM/SQL storage.
func (e ErrorText) Value() (driver.Value, error) {
	if e == "" {
		return nil, nil
	}
	return string(e), nil
}

// Scan implements sql.Scanner for GORM/SQL loading.
func (e *ErrorText) Scan(value any) error {
	if e == nil {
		return fmt.Errorf("journal.ErrorText: Scan on nil pointer")
	}
	if value == nil {
		*e = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*e = ErrorText(v)
	case []byte:
		*e = ErrorText(v)
	default:
		return fmt.Errorf("journal.ErrorText: cannot scan %T", value)
	}
	return nil
}

// JournalError is a journal domain error that carries a traceback id.
type JournalError interface {
	error

	TracebackID() uuid.UUID
}

func generateTracebackID() uuid.UUID {
	return uuid.New()
}

// AlreadyRegistered indicates a request id is already present in the journal.
type AlreadyRegistered struct {
	tracebackID uuid.UUID
	Request     Request
}

// NewAlreadyRegistered creates an AlreadyRegistered error for request.
func NewAlreadyRegistered(request Request) AlreadyRegistered {
	return AlreadyRegistered{
		tracebackID: generateTracebackID(),
		Request:     request,
	}
}

// NewAlreadyRegisteredWithTracebackID creates an AlreadyRegistered error with a fixed traceback id.
func NewAlreadyRegisteredWithTracebackID(request Request, tracebackID uuid.UUID) AlreadyRegistered {
	return AlreadyRegistered{
		tracebackID: tracebackID,
		Request:     request,
	}
}

func (e AlreadyRegistered) Error() string {
	return fmt.Sprintf("request (%s from group %s) is already registered", e.Request.ID, e.Request.GroupID)
}

// TracebackID returns the error traceback identifier.
func (e AlreadyRegistered) TracebackID() uuid.UUID {
	return e.tracebackID
}

// NotFound indicates a request id is missing from the journal.
type NotFound struct {
	tracebackID    uuid.UUID
	GroupRequestID ID
	RequestID      ID
}

// NewNotFound creates a NotFound error for id.
func NewNotFound(requestID, groupRequestID ID) NotFound {
	return NotFound{
		tracebackID:    generateTracebackID(),
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
	}
}

// NewNotFoundWithTracebackID creates a NotFound error with a fixed traceback id.
func NewNotFoundWithTracebackID(requestID, groupRequestID ID, tracebackID uuid.UUID) NotFound {
	return NotFound{
		tracebackID:    tracebackID,
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
	}
}

func (e NotFound) Error() string {
	return fmt.Sprintf("request (%s from group %s) not found", e.RequestID, e.GroupRequestID)
}

// TracebackID returns the error traceback identifier.
func (e NotFound) TracebackID() uuid.UUID {
	return e.tracebackID
}

// StatusConflict indicates an illegal status transition for a request.
type StatusConflict struct {
	tracebackID    uuid.UUID
	RequestID      ID
	GroupRequestID ID
	Actual         Status
	Got            Status
}

// NewStatusConflict creates a StatusConflict for attempting to set got while actual is present.
func NewStatusConflict(requestID, groupRequestID ID, got, actual Status) StatusConflict {
	return StatusConflict{
		tracebackID:    generateTracebackID(),
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
		Actual:         actual,
		Got:            got,
	}
}

// NewStatusConflictWithTracebackID creates a StatusConflict with a fixed traceback id.
func NewStatusConflictWithTracebackID(requestID, groupRequestID ID, got, actual Status, tracebackID uuid.UUID) StatusConflict {
	return StatusConflict{
		tracebackID:    tracebackID,
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
		Actual:         actual,
		Got:            got,
	}
}

func (e StatusConflict) Error() string {
	return fmt.Sprintf("attempt to set %v status, but request (%s from group %s) has %v status", e.Got, e.RequestID, e.GroupRequestID, e.Actual)
}

// TracebackID returns the error traceback identifier.
func (e StatusConflict) TracebackID() uuid.UUID {
	return e.tracebackID
}

// InternalErr wraps an unexpected journal infrastructure failure.
type InternalErr struct {
	tracebackID uuid.UUID
	err         error
}

// NewInternalErr creates an InternalErr around err.
func NewInternalErr(err error) InternalErr {
	return InternalErr{
		tracebackID: generateTracebackID(),
		err:         err,
	}
}

// NewInternalErrWithTracebackID creates an InternalErr with a fixed traceback id.
func NewInternalErrWithTracebackID(err error, tracebackID uuid.UUID) InternalErr {
	return InternalErr{
		tracebackID: tracebackID,
		err:         err,
	}
}

func (e InternalErr) Error() string {
	return fmt.Sprintf("journal internal error: %v", e.err)
}

// TracebackID returns the error traceback identifier.
func (e InternalErr) TracebackID() uuid.UUID {
	return e.tracebackID
}

// Unwrap returns the underlying error.
func (e InternalErr) Unwrap() error {
	return e.err
}
