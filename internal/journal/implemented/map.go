// Package implemented provides concrete Journal implementations.
package implemented

import (
	"context"
	"sync"
	"time"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"gorm.io/gorm"
)

var (
	_ journal.Journal = (*MapJournal)(nil)
)

// MapJournal is an in-memory Journal backed by a map keyed by group then request id.
type MapJournal struct {
	m  map[libjournal.ID]*MapGroup
	mu sync.RWMutex
}

// NewMapJournal creates an empty MapJournal.
func NewMapJournal() *MapJournal {
	return &MapJournal{
		m: make(map[libjournal.ID]*MapGroup),
	}
}

// Register stores a running request under groupRequestID and returns Complete/Fail callbacks.
// Returns AlreadyRegistered if requestID is already present, or ErrGroupIsClosed if the group is closed.
func (mj *MapJournal) Register(_ context.Context, requestID, groupRequestID libjournal.ID, tool string, argsHash []byte) (journal.CompleteFunc, journal.FailFunc, error) {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	if _, ok := mj.m[groupRequestID]; !ok {
		mj.m[groupRequestID] = NewMapGroup(groupRequestID)
	}

	group := mj.m[groupRequestID]
	if request, ok := group.Get(requestID); ok {
		return nil, nil, journal.NewAlreadyRegistered(cloneRequest(request))
	}

	if err := group.Add(libjournal.Request{
		ID:        requestID,
		GroupID:   groupRequestID,
		CreatedAt: time.Now(),
		Tool:      tool,
		ArgsHash:  argsHash,
		Status:    libjournal.Status_Running,
	}); err != nil {
		return nil, nil, err
	}

	return mj.Complete, mj.Fail, nil
}

// Complete marks a running request as completed and stores result.
// Returns NotFound if the request is missing, or StatusConflict if it is not running.
func (mj *MapJournal) Complete(_ context.Context, requestID, groupRequestID libjournal.ID, result libjournal.Result, _ *gorm.DB) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	group, ok := mj.m[groupRequestID]
	if !ok {
		return journal.NewNotFound(requestID, groupRequestID)
	}

	request, ok := group.Get(requestID)
	if !ok {
		return journal.NewNotFound(requestID, groupRequestID)
	}

	if request.Status != libjournal.Status_Running {
		return journal.NewStatusConflict(requestID, groupRequestID, libjournal.Status_Completed, request.Status)
	}

	request.Status = libjournal.Status_Completed
	request.Result = result
	return group.Swap(request)
}

// Fail marks a running request as failed and stores cause.
// Returns NotFound if the request is missing, or StatusConflict if it is not running.
func (mj *MapJournal) Fail(_ context.Context, requestID, groupRequestID libjournal.ID, cause error, _ *gorm.DB) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	group, ok := mj.m[groupRequestID]
	if !ok {
		return journal.NewNotFound(requestID, groupRequestID)
	}

	request, ok := group.Get(requestID)
	if !ok {
		return journal.NewNotFound(requestID, groupRequestID)
	}

	if request.Status != libjournal.Status_Running {
		return journal.NewStatusConflict(requestID, groupRequestID, libjournal.Status_Failed, request.Status)
	}

	request.Status = libjournal.Status_Failed
	request.Error = libjournal.ErrorTextFrom(cause)
	return group.Swap(request)
}

// Find returns a copy of the request for requestID within groupRequestID.
// Returns NotFound if the group or request is missing.
func (mj *MapJournal) Find(_ context.Context, requestID, groupRequestID libjournal.ID) (libjournal.Request, error) {
	mj.mu.RLock()
	defer mj.mu.RUnlock()

	group, ok := mj.m[groupRequestID]
	if !ok {
		return libjournal.Request{}, journal.NewNotFound(requestID, groupRequestID)
	}

	if request, ok := group.Get(requestID); ok {
		return cloneRequest(request), nil
	}

	return libjournal.Request{}, journal.NewNotFound(requestID, groupRequestID)
}

// Close sets ExpiresAt on every request in the group and rejects new registrations.
// Missing groups are ignored.
func (mj *MapJournal) Close(_ context.Context, groupRequestID libjournal.ID, deadline time.Time) error {
	mj.mu.Lock()
	defer mj.mu.Unlock()

	group, ok := mj.m[groupRequestID]
	if !ok {
		return nil
	}

	group.Close(deadline)

	return nil
}

// IsGroupFree reports whether the group has no requests.
// Returns false if at least one request exists, regardless of expiry.
func (mj *MapJournal) IsGroupFree(_ context.Context, groupRequestID libjournal.ID) (bool, error) {
	mj.mu.RLock()
	defer mj.mu.RUnlock()

	group, ok := mj.m[groupRequestID]
	if !ok {
		return true, nil
	}

	return group.Len() == 0, nil
}

// DeleteAllExpired removes every group for which all requests are expired under hardTTL.
// Respects ctx cancellation before and between groups.
func (mj *MapJournal) DeleteAllExpired(ctx context.Context, hardTTL time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	mj.mu.Lock()
	defer mj.mu.Unlock()

	now := time.Now()
	for groupID, group := range mj.m {
		if err := ctx.Err(); err != nil {
			return err
		}
		if group.IsExpired(now, hardTTL) {
			delete(mj.m, groupID)
		}
	}

	return nil
}
