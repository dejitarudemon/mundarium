package journal

import (
	"errors"
	"fmt"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/google/uuid"
)

var (
	_ JournalError = AlreadyRegistered{}
	_ JournalError = NotFound{}
	_ JournalError = StatusConflict{}
)

var (
	// ErrGroupIsClosed is returned when registering into a group that was already closed.
	ErrGroupIsClosed = errors.New("group is closed")
)

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
	Request     libjournal.Request
}

// NewAlreadyRegistered creates an AlreadyRegistered error for request.
func NewAlreadyRegistered(request libjournal.Request) AlreadyRegistered {
	return AlreadyRegistered{
		tracebackID: generateTracebackID(),
		Request:     request,
	}
}

// NewAlreadyRegisteredWithTracebackID creates an AlreadyRegistered error with a fixed traceback id.
func NewAlreadyRegisteredWithTracebackID(request libjournal.Request, tracebackID uuid.UUID) AlreadyRegistered {
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
	GroupRequestID libjournal.RequestID
	RequestID      libjournal.RequestID
}

// NewNotFound creates a NotFound error for id.
func NewNotFound(requestID, groupRequestID libjournal.RequestID) NotFound {
	return NotFound{
		tracebackID:    generateTracebackID(),
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
	}
}

// NewNotFoundWithTracebackID creates a NotFound error with a fixed traceback id.
func NewNotFoundWithTracebackID(requestID, groupRequestID libjournal.RequestID, tracebackID uuid.UUID) NotFound {
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
	RequestID      libjournal.RequestID
	GroupRequestID libjournal.RequestID
	Actual         libjournal.Status
	Got            libjournal.Status
}

// NewStatusConflict creates a StatusConflict for attempting to set got while actual is present.
func NewStatusConflict(requestID, groupRequestID libjournal.RequestID, got, actual libjournal.Status) StatusConflict {
	return StatusConflict{
		tracebackID:    generateTracebackID(),
		RequestID:      requestID,
		GroupRequestID: groupRequestID,
		Actual:         actual,
		Got:            got,
	}
}

// NewStatusConflictWithTracebackID creates a StatusConflict with a fixed traceback id.
func NewStatusConflictWithTracebackID(requestID, groupRequestID libjournal.RequestID, got, actual libjournal.Status, tracebackID uuid.UUID) StatusConflict {
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
