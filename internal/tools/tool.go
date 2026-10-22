// Package tools defines the executable tool interface used by agents and orchestration.
package tools

import (
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

// Tool is a named action with a JSON schema definition and an Execute entrypoint.
type Tool interface {
	Definition() definition.Definition
	Execute(ctx execution.ExecutionContext, arguments json.RawMessage) (json.RawMessage, error)
}
