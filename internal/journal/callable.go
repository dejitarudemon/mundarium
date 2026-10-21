package journal

import (
	"context"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// CompleteFunc is invoked when an operation finishes successfully enough to report a Result.
type CompleteFunc func(ctx context.Context, id libjournal.RequestID, result Result) error

// FailFunc is invoked when an operation fails and should be recorded as failed.
type FailFunc func(ctx context.Context, id libjournal.RequestID, cause error) error
