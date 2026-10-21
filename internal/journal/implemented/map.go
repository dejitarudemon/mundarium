// Package implemented provides concrete Journal implementations.
package implemented

import (
	"context"
	"sync"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

var (
	_ journal.Journal = (*MapJournal)(nil)
)

// MapJournal is an in-memory Journal backed by a map.
type MapJournal struct {
	m  map[libjournal.RequestID]*journal.Request
	mu sync.RWMutex
}

// NewMapJournal creates an empty MapJournal.
func NewMapJournal() *MapJournal {
	return &MapJournal{
		m: make(map[libjournal.RequestID]*journal.Request),
	}
}

// Register stores a running request for id and returns Complete/Fail callbacks.
// Returns AlreadyRegistered if id is already present.
func (mj *MapJournal) Register(_ context.Context, id libjournal.RequestID, action libjournal.Action) (journal.CompleteFunc, journal.FailFunc, error) {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	if request, ok := mj.m[id]; ok {
		return nil, nil, journal.NewAlreadyRegistered(*request)
	}

	mj.m[id] = &journal.Request{
		ID:     id,
		Action: action,
		Status: libjournal.Status_Running,
	}

	return mj.Complete, mj.Fail, nil
}

// Complete marks the request as completed and stores result.
// Returns NotFound if id is missing, or StatusConflict if the request already failed.
func (mj *MapJournal) Complete(_ context.Context, id libjournal.RequestID, result journal.Result) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	request, ok := mj.m[id]
	if !ok {
		return journal.NewNotFound(id)
	}

	if request.Status == libjournal.Status_Failed {
		return journal.NewStatusConflict(id, libjournal.Status_Completed, request.Status)
	}

	request.Status = libjournal.Status_Completed
	request.Result = &result

	return nil
}

// Fail marks the request as failed and stores cause.
// Returns NotFound if id is missing, or StatusConflict if the request already failed.
func (mj *MapJournal) Fail(_ context.Context, id libjournal.RequestID, cause error) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	request, ok := mj.m[id]
	if !ok {
		return journal.NewNotFound(id)
	}

	if request.Status == libjournal.Status_Failed {
		return journal.NewStatusConflict(id, libjournal.Status_Failed, request.Status)
	}

	request.Status = libjournal.Status_Failed
	request.Error = cause

	return nil
}

// Find returns a copy of the request for id.
// Returns NotFound if id is missing.
func (mj *MapJournal) Find(_ context.Context, id libjournal.RequestID) (journal.Request, error) {
	mj.mu.RLock()
	defer mj.mu.RUnlock()

	request, ok := mj.m[id]
	if !ok {
		return journal.Request{}, journal.NewNotFound(id)
	}

	return *request, nil
}

// Delete removes the request for id. Missing ids are ignored.
func (mj *MapJournal) Delete(_ context.Context, id libjournal.RequestID) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	delete(mj.m, id)
	return nil
}
