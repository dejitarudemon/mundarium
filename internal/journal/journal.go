package journal

import (
	"context"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// Journal records tool invocations associated with a request.
type Journal interface {
	// Register stores a running request for id under the given tool name and args hash.
	// argsHash is an opaque digest (blake2b) computed by the caller; the journal stores it as-is.
	Register(ctx context.Context, id libjournal.RequestID, tool string, argsHash []byte) (CompleteFunc, FailFunc, error)

	// Find returns the request record for id.
	Find(ctx context.Context, id libjournal.RequestID) (Request, error)

	// Delete removes the request record for id.
	Delete(ctx context.Context, id libjournal.RequestID) error
}
