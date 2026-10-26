package implemented

import (
	"sync"
	"time"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// MapGroup is a thread-safe in-memory store for requests that share one group id.
// After Close, Add rejects new registrations; Swap may still update existing ones.
type MapGroup struct {
	m      map[libjournal.ID]libjournal.Request
	closed bool

	id libjournal.ID

	mx sync.RWMutex
}

// NewMapGroup creates an empty open group bound to id.
func NewMapGroup(id libjournal.ID) *MapGroup {
	return &MapGroup{
		m:  make(map[libjournal.ID]libjournal.Request),
		id: id,
	}
}

// Add inserts a new request by ID.
// Returns ErrGroupIsClosed if the group was closed, or AlreadyRegistered if the ID exists.
func (g *MapGroup) Add(request libjournal.Request) error {
	g.mx.Lock()
	defer g.mx.Unlock()

	if g.closed {
		return journal.ErrGroupIsClosed
	}

	if request, exists := g.m[request.ID]; exists {
		return journal.NewAlreadyRegistered(request)
	}

	g.m[request.ID] = cloneRequest(request)
	return nil
}

// Swap replaces an existing request by ID and preserves its ExpiresAt.
// Returns NotFound if the ID is missing. Allowed after Close so Complete/Fail can finish.
func (g *MapGroup) Swap(request libjournal.Request) error {
	g.mx.Lock()
	defer g.mx.Unlock()

	if _, exists := g.m[request.ID]; !exists {
		return journal.NewNotFound(request.ID, g.id)
	}

	request = cloneRequest(request)

	request.ExpiresAt = g.m[request.ID].ExpiresAt
	g.m[request.ID] = cloneRequest(request)
	return nil
}

// Get returns a copy of the request for id, or false if missing.
func (g *MapGroup) Get(id libjournal.ID) (libjournal.Request, bool) {
	g.mx.RLock()
	defer g.mx.RUnlock()

	request, ok := g.m[id]
	if !ok {
		return libjournal.Request{}, false
	}
	return cloneRequest(request), true
}

// Close sets ExpiresAt on every request to deadline and marks the group closed.
// Idempotent: a second Close does not rewrite ExpiresAt.
func (g *MapGroup) Close(deadline time.Time) {
	g.mx.Lock()
	defer g.mx.Unlock()

	if !g.closed {
		for id, request := range g.m {
			request.ExpiresAt = &deadline
			g.m[id] = request
		}
	}

	g.closed = true
}

// Len returns the number of requests currently in the group.
func (g *MapGroup) Len() int {
	g.mx.RLock()
	defer g.mx.RUnlock()

	return len(g.m)
}

// IsExpired reports whether every request is expired under soft ExpiresAt and/or hard ttl.
// Returns false for an empty group.
func (g *MapGroup) IsExpired(now time.Time, ttl time.Duration) bool {
	g.mx.RLock()
	defer g.mx.RUnlock()

	if len(g.m) == 0 {
		return false
	}
	for _, request := range g.m {
		if !request.IsExpired(now, ttl) {
			return false
		}
	}
	return true
}

func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	out := make([]byte, len(b))
	copy(out, b)
	return out
}

func cloneRequest(req libjournal.Request) libjournal.Request {
	req.ArgsHash = cloneBytes(req.ArgsHash)
	if req.ExpiresAt != nil {
		t := *req.ExpiresAt
		req.ExpiresAt = &t
	}
	return req
}
