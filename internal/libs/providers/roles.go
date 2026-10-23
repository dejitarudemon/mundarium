// Package providers holds caller identity primitives used by tools and the executor.
package providers

// Role identifies who may invoke a tool.
type Role string

const (
	Role_Master Role = "master" // full access to registered tools that allow this role
)
