// Package services provides shared execution context for application services.
package services

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/journal"
)

// ExecutionContext carries request-scoped context, a journal request ID, and an optional completion callback.
type ExecutionContext struct {
	context.Context

	requestID        journal.RequestID
	callbackComplete journal.CompleteFunc
}

// NewExecutionContext creates an ExecutionContext with a required completion callback.
// Returns an error if callback is nil. A nil parent is replaced with context.Background.
func NewExecutionContext(parent context.Context, requestID journal.RequestID, callback journal.CompleteFunc) (ExecutionContext, context.CancelFunc, error) {
	if callback == nil {
		return ExecutionContext{}, nil, errors.New("nil callback")
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext{
		Context:          ctx,
		requestID:        requestID,
		callbackComplete: callback,
	}, cancel, nil
}

// NewExecutionContextWithoutCallback creates an ExecutionContext without a completion callback.
// Complete becomes a no-op. A nil parent is replaced with context.Background.
func NewExecutionContextWithoutCallback(parent context.Context, requestID journal.RequestID) (ExecutionContext, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext{
		Context:   ctx,
		requestID: requestID,
	}, cancel
}

// Complete reports result through the configured callback, if any.
// When no callback is set, Complete returns nil.
func (e ExecutionContext) Complete(result journal.Result) error {
	if e.callbackComplete == nil {
		return nil
	}

	return e.callbackComplete(e.Context, e.requestID, result)
}
