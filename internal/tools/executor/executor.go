package executor

import (
	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/tools"
)

const IDSep = ":"

type Executor struct {
	tools   map[string]tools.Tool
	journal journal.Journal
}

func NewExecutor(journal journal.Journal) (*Executor, error) {
	if journal == nil {
		return nil, ErrJournalIsNil
	}

	return &Executor{
		tools:   map[string]tools.Tool{},
		journal: journal,
	}, nil
}

func (e *Executor) Register(tool tools.Tool) error {
	if tool == nil {
		return ErrToolIsNil
	}

	name := tool.Definition().Name
	if len(name) == 0 {
		return ErrNoName
	}

	if _, ok := e.tools[name]; ok {
		return ErrToolExists
	}

	e.tools[name] = tool
	return nil
}
