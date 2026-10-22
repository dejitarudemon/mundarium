package executor

import "errors"

var (
	ErrJournalIsNil = errors.New("journal is nil")
	ErrToolIsNil    = errors.New("tool is nil")
	ErrToolExists   = errors.New("tool already registered")
	ErrNoName       = errors.New("tool has no name")
	ErrToolNotFound = errors.New("tool not found")

	// ErrRequestToolMismatch is returned when a request id is already registered under another tool name.
	ErrRequestToolMismatch = errors.New("request already registered for a different tool")
	// ErrRequestStillRunning is returned when a non-idempotent request is still in progress.
	ErrRequestStillRunning = errors.New("request is still running")
	// ErrRequestAlreadyFailed is returned when a non-idempotent request previously failed without a cause.
	ErrRequestAlreadyFailed = errors.New("request already failed")
	// ErrUnexpectedRequestStatus is returned when a journaled request has an unknown status.
	ErrUnexpectedRequestStatus = errors.New("unexpected request status")
)
