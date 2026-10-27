package errs

import "errors"

var (
	ErrJournalIsNil = errors.New("journal is nil")
	ErrToolIsNil    = errors.New("tool is nil")
	ErrToolExists   = errors.New("tool already registered")
	ErrNoName       = errors.New("tool has no name")

	// ErrServiceIsNil is returned when a tool constructor receives a nil service dependency.
	ErrServiceIsNil = errors.New("expected service, got nil")

	ErrLocationIDZero   = errors.New("location id is zero")
	ErrParentIDZero     = errors.New("parent id is zero")
	ErrConnectionIDZero = errors.New("connection id is zero")
	ErrToIDZero         = errors.New("to id is zero")
)
