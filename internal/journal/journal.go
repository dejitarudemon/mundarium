package journal

import (
	"context"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// Journal records service operations associated with a request.
type Journal interface {
	// Register stores that the given action was performed for request id.
	Register(ctx context.Context, id RequestID, action libjournal.Action) (CompleteFunc, FailFunc, error)

	// Find returns the request record for id.
	Find(ctx context.Context, id RequestID) (Request, error)

	// Delete removes the request record for id.
	Delete(ctx context.Context, id RequestID) error
}
