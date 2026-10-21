// Package journal defines types used to correlate and record service operations.
package journal

import (
	"github.com/google/uuid"
)

// RequestID identifies a single client or orchestrator request in the journal.
type RequestID uuid.UUID
