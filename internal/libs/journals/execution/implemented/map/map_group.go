package mapjournal

import (
	"sync"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

// Group is a thread-safe in-memory store for requests that share one group id.
// After Close, Add rejects new registrations; Swap may still update existing ones.
type Group struct {
	m      map[journal.ID]journal.Request
	closed bool

	id journal.ID

	mx sync.RWMutex
}

// NewGroup creates an empty open group bound to id.
func NewGroup(id journal.ID) *Group {
	return &Group{
		m:  make(map[journal.ID]journal.Request),
		id: id,
	}
}

// Add inserts a new request by ID.
// Returns ErrGroupIsClosed if the group was closed, or AlreadyRegistered if the ID exists.
func (g *Group) Add(request journal.Request) error {
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
func (g *Group) Swap(request journal.Request) error {
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
func (g *Group) Get(id journal.ID) (journal.Request, bool) {
	g.mx.RLock()
	defer g.mx.RUnlock()

	request, ok := g.m[id]
	if !ok {
		return journal.Request{}, false
	}
	return cloneRequest(request), true
}

// Close sets ExpiresAt on every request to deadline and marks the group closed.
// Idempotent: a second Close does not rewrite ExpiresAt.
func (g *Group) Close(deadline time.Time) {
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
func (g *Group) Len() int {
	g.mx.RLock()
	defer g.mx.RUnlock()

	return len(g.m)
}

// IsExpired reports whether every request is expired under soft ExpiresAt and/or hard ttl.
// Returns false for an empty group.
func (g *Group) IsExpired(now time.Time, ttl time.Duration) bool {
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

func cloneRequest(req journal.Request) journal.Request {
	req.ArgsHash = cloneBytes(req.ArgsHash)
	if req.ExpiresAt != nil {
		t := *req.ExpiresAt
		req.ExpiresAt = &t
	}
	return req
}
