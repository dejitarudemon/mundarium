// Package providers defines shared provider request primitives.
package providers

import (
	"bytes"
	"encoding/json"
)

// Request is an opaque provider invocation payload.
type Request struct {
	Args json.RawMessage `json:"args"`
}

// Copy returns a deep copy of the request so callers can mutate Args safely.
func (r Request) Copy() Request {
	return Request{Args: bytes.Clone(r.Args)}
}
