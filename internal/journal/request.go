package journal

import libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"

// Request is a journaled tool invocation and its outcome.
// Result is set when Status is completed; Error is set when Status is failed.
type Request struct {
	ID libjournal.RequestID

	Status   libjournal.Status
	Tool     string // tool name, e.g. "create_location"
	ArgsHash []byte // blake2b digest of tool args; computed by the executor

	Result Result
	Error  error
}
