// Package tools defines shared tool call primitives.
package tools

import (
	"encoding/json"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

// Call is a single tool invocation requested by an agent or orchestrator.
type Call struct {
	ID   string          // call id within an invoke
	Name string          // tool name
	Role types.Role      // caller role used for tool permission checks
	Args json.RawMessage // tool arguments
}
