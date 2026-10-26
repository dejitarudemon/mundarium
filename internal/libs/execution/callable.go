package execution

import (
	"context"

	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

// CompleteFunc records a successful outcome for requestID within groupRequestID.
// tx is the optional GORM transaction the caller wants associated with the journal write.
type CompleteFunc[E any] func(ctx context.Context, requestID, groupRequestID types.ID, result E, tx *gorm.DB) error

// FailFunc records a failure for requestID within groupRequestID.
// tx is the optional GORM transaction the caller wants associated with the journal write.
type FailFunc func(ctx context.Context, requestID, groupRequestID types.ID, cause error, tx *gorm.DB) error
