package journal

import (
	"context"
	"time"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// Journal records tool invocations grouped by a shared group request id.
type Journal interface {
	// Register stores a running request under groupRequestID with the given tool name and args hash.
	// argsHash is an opaque digest (blake2b) computed by the caller; the journal stores it as-is.
	// Returns AlreadyRegistered if requestID is already present, or ErrGroupIsClosed if the group is closed.
	Register(ctx context.Context, requestID, groupRequestID libjournal.ID, tool string, argsHash []byte) (CompleteFunc, FailFunc, error)

	// Find returns the request record for requestID within groupRequestID.
	Find(ctx context.Context, requestID, groupRequestID libjournal.ID) (libjournal.Request, error)

	// DeleteAllExpired removes every group whose requests are all expired under hardTTL.
	// Walks all groups and respects ctx cancellation.
	DeleteAllExpired(ctx context.Context, hardTTL time.Duration) error

	// Close sets ExpiresAt on every request in the group and rejects new registrations.
	Close(ctx context.Context, groupRequestID libjournal.ID, deadline time.Time) error

	// IsGroupFree reports whether the group has no requests.
	// Returns false if at least one request exists in the group, regardless of expiry.
	IsGroupFree(ctx context.Context, groupRequestID libjournal.ID) (bool, error)
}
