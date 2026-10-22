package journal

import (
	"context"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// Journal records tool invocations associated with a request.
type Journal interface {
	// Register stores a running request for id under the given tool name.
	Register(ctx context.Context, id libjournal.RequestID, tool string) (CompleteFunc, FailFunc, error)

	// Find returns the request record for id.
	Find(ctx context.Context, id libjournal.RequestID) (Request, error)

	// Delete removes the request record for id.
	Delete(ctx context.Context, id libjournal.RequestID) error
}
