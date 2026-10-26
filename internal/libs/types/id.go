package types

// ID identifies a call, invoke/group, or other opaque string key in the runtime.
type ID string

// String returns the id as a plain string.
func (id ID) String() string {
	return string(id)
}
