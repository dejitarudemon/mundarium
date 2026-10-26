// Package tools defines the executable tool interface used by agents and orchestration.
package tools

import (
	"encoding/json"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

// Tool is a named action with a JSON schema definition and an Execute entrypoint.
type Tool interface {
	Definition() definition.Definition
	Execute(ctx execution.ExecutionContext[journal.Result], arguments json.RawMessage) (json.RawMessage, error)
	// IsIdempotent reports whether repeated Execute calls with the same request are safe
	// without journal registration (e.g. pure reads).
	IsIdempotent() bool
}
