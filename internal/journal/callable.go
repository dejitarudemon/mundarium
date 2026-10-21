package journal

import "context"

// CompleteFunc is invoked when an operation finishes successfully enough to report a Result.
type CompleteFunc func(ctx context.Context, id RequestID, result Result) error

// FailFunc is invoked when an operation fails and should be recorded as failed.
type FailFunc func(ctx context.Context, id RequestID, cause error) error
