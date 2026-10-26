// Package journal defines the execution journal: request lifecycle,
// completion callbacks, and shared domain primitives (Action, Status, Request, Result).
package journal

// Action is the kind of operation recorded for journaling callbacks.
type Action string

// Known journal actions.
const (
	Action_Create Action = "create"
	Action_Get    Action = "get"
	Action_Patch  Action = "patch"
	Action_Update Action = "update"
	Action_Delete Action = "delete"
)

// String returns the canonical action name.
func (a Action) String() string {
	return string(a)
}
