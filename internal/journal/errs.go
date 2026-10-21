package journal

import (
	"fmt"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/google/uuid"
)

var (
	_ JournalError = AlreadyRegistered{}
	_ JournalError = NotFound{}
	_ JournalError = StatusConflict{}
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
	return fmt.Sprintf("request (%s) is already registered", e.Request.ID)
}

// TracebackID returns the error traceback identifier.
func (e AlreadyRegistered) TracebackID() uuid.UUID {
	return e.tracebackID
}

// NotFound indicates a request id is missing from the journal.
type NotFound struct {
	tracebackID uuid.UUID
	ID          libjournal.RequestID
}

// NewNotFound creates a NotFound error for id.
func NewNotFound(id libjournal.RequestID) NotFound {
	return NotFound{
		tracebackID: generateTracebackID(),
		ID:          id,
	}
}

// NewNotFoundWithTracebackID creates a NotFound error with a fixed traceback id.
func NewNotFoundWithTracebackID(id libjournal.RequestID, tracebackID uuid.UUID) NotFound {
	return NotFound{
		tracebackID: tracebackID,
		ID:          id,
	}
}

func (e NotFound) Error() string {
	return fmt.Sprintf("request (%s) not found", e.ID)
}

// TracebackID returns the error traceback identifier.
func (e NotFound) TracebackID() uuid.UUID {
	return e.tracebackID
}

// StatusConflict indicates an illegal status transition for a request.
type StatusConflict struct {
	tracebackID uuid.UUID
	ID          libjournal.RequestID
	Actual      libjournal.Status
	Got         libjournal.Status
}

// NewStatusConflict creates a StatusConflict for attempting to set got while actual is present.
func NewStatusConflict(id libjournal.RequestID, got, actual libjournal.Status) StatusConflict {
	return StatusConflict{
		tracebackID: generateTracebackID(),
		ID:          id,
		Actual:      actual,
		Got:         got,
	}
}

// NewStatusConflictWithTracebackID creates a StatusConflict with a fixed traceback id.
func NewStatusConflictWithTracebackID(id libjournal.RequestID, got, actual libjournal.Status, tracebackID uuid.UUID) StatusConflict {
	return StatusConflict{
		tracebackID: tracebackID,
		ID:          id,
		Actual:      actual,
		Got:         got,
	}
}

func (e StatusConflict) Error() string {
	return fmt.Sprintf("attempt to set %v status, but request (%s) has %v status", e.Got, e.ID, e.Actual)
}

// TracebackID returns the error traceback identifier.
func (e StatusConflict) TracebackID() uuid.UUID {
	return e.tracebackID
}
