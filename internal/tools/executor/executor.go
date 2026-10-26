package executor

import (
	"sync"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

type Executor struct {
	tools   map[string]tools.Tool
	journal journal.Journal
	allowed map[types.Role][]definition.Definition

	mx sync.RWMutex
}

func NewExecutor(journal journal.Journal) (*Executor, error) {
	if journal == nil {
		return nil, toolerrs.ErrJournalIsNil
	}

	return &Executor{
		tools:   map[string]tools.Tool{},
		journal: journal,
		allowed: make(map[types.Role][]definition.Definition),
	}, nil
}

// Register adds tool and grants it to each role in allowed.
func (e *Executor) Register(tool tools.Tool, allowed []types.Role) error {
	if tool == nil {
		return toolerrs.ErrToolIsNil
	}

	name := tool.Definition().Name
	if len(name) == 0 {
		return toolerrs.ErrNoName
	}

	e.mx.Lock()
	defer e.mx.Unlock()

	if _, ok := e.tools[name]; ok {
		return toolerrs.ErrToolExists
	}

	e.tools[name] = tool

	def := tool.Definition().Copy()
	for _, role := range allowed {
		e.allowed[role] = append(e.allowed[role], def.Copy())
	}

	return nil
}

// Allowed returns a copy of tool definitions registered for role.
func (e *Executor) Allowed(role types.Role) []definition.Definition {
	e.mx.RLock()
	defer e.mx.RUnlock()

	defs := e.allowed[role]
	if len(defs) == 0 {
		return nil
	}

	result := make([]definition.Definition, 0, len(defs))

	for _, def := range defs {
		result = append(result, def.Copy())
	}

	return result
}
