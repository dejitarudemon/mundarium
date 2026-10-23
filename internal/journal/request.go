package journal

import libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"

// Request is a journaled tool invocation and its outcome.
type Request = libjournal.Request

// Result describes a completed service operation for journaling or callbacks.
type Result = libjournal.Result
