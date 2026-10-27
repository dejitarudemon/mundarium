package types

// Role identifies who may invoke a tool.
type Role string

const (
	Role_Master Role = "master" // full access to registered tools that allow this role
)

// String returns the canonical role name (for example "master").
func (r Role) String() string {
	return string(r)
}
