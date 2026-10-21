package journal

import libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"

// Request is a journaled service request and its outcome.
// Result is set when Status is completed; Error is set when Status is failed.
type Request struct {
	ID libjournal.RequestID

	Action libjournal.Action
	Status libjournal.Status

	Result Result
	Error  error
}
