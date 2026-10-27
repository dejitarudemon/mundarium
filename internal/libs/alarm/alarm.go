// Package alarm provides a context-aware one-shot signal between goroutines.
package alarm

import (
	"context"
)

// Alarm is a buffered signal: Ring notifies, Wait receives.
type Alarm struct {
	a chan struct{}
}

// NewAlarm creates an Alarm with a buffer of at least 1.
// preallocate caps how many Rings can complete without a Wait.
func NewAlarm(preallocate int) *Alarm {
	return &Alarm{a: make(chan struct{}, max(1, preallocate))}
}

// Ring sends a signal or returns ctx.Err() if ctx is done first.
// Blocks if the buffer is full until a Wait receives or ctx is done.
func (a *Alarm) Ring(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case a.a <- struct{}{}:
	}

	return nil
}

// Wait receives one signal or returns ctx.Err() if ctx is done first.
func (a *Alarm) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-a.a:
	}

	return nil
}
