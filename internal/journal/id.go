package journal

// ID identifies a journaled request or invoke/group in the journal and execution context.
type ID string

// String returns the id as a plain string.
func (id ID) String() string {
	return string(id)
}
