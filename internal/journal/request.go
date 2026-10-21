package journal

import libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"

// Request is a journaled service request and its outcome.
type Request struct {
	ID RequestID

	Action libjournal.Action
	Status libjournal.Status

	Result *Result
	Error  error
}
