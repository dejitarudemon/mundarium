package journal

import (
	"github.com/google/uuid"
)

// RequestID identifies a single client or orchestrator request in the journal.
type RequestID uuid.UUID

// String returns the canonical UUID representation of the request id.
func (id RequestID) String() string {
	return uuid.UUID(id).String()
}
