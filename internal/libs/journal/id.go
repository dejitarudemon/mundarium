package journal

// RequestID identifies a single client or orchestrator request in the journal.
// The same type is also used for invoke/group ids in execution context.
type RequestID string

// String returns the id as a plain string.
func (id RequestID) String() string {
	return string(id)
}
