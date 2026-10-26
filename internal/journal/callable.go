package journal

import (
	"context"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"gorm.io/gorm"
)

// CompleteFunc records a successful outcome for requestID within groupRequestID.
// tx is the optional GORM transaction the caller wants associated with the journal write.
type CompleteFunc func(ctx context.Context, requestID, groupRequestID libjournal.ID, result libjournal.Result, tx *gorm.DB) error

// FailFunc records a failure for requestID within groupRequestID.
// tx is the optional GORM transaction the caller wants associated with the journal write.
type FailFunc func(ctx context.Context, requestID, groupRequestID libjournal.ID, cause error, tx *gorm.DB) error
