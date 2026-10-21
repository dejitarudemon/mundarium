package services

// Result describes a completed service operation for journaling or callbacks.
type Result struct {
	Operation string // operation name, e.g. "create", "get", "patch"
	Data      any    // operation payload
	Entity    string // entity kind, e.g. "LOCATION"
}
