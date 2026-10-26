package journal

import "github.com/dejitarudemon/mundarium/internal/libs/execution"

// CompleteFunc records a successful journal outcome for a request within a group.
type CompleteFunc = execution.CompleteFunc[Result]

// FailFunc records a failed journal outcome for a request within a group.
type FailFunc = execution.FailFunc
