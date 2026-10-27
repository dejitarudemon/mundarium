package postgres

import (
	"sync"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

type key struct {
	requestID journal.ID
	groupID   journal.ID
}

func newKey(requestID, groupID journal.ID) key {
	return key{
		requestID: requestID,
		groupID:   groupID,
	}
}

// emergency tracks request ids whose Fail could not be persisted to Postgres.
type emergency struct {
	m  map[key]struct{}
	mx sync.RWMutex
}

func newEmergency() *emergency {
	return &emergency{
		m: make(map[key]struct{}),
	}
}

func (e *emergency) add(requestID, groupID journal.ID) {
	e.mx.Lock()
	defer e.mx.Unlock()

	e.m[newKey(requestID, groupID)] = struct{}{}
}

func (e *emergency) find(requestID, groupID journal.ID) bool {
	e.mx.RLock()
	defer e.mx.RUnlock()

	_, ok := e.m[newKey(requestID, groupID)]
	return ok
}

func (e *emergency) delete(groupID journal.ID) {
	e.mx.Lock()
	defer e.mx.Unlock()

	for k := range e.m {
		if k.groupID == groupID {
			delete(e.m, k)
		}
	}
}
