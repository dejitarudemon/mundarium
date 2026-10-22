// Package execution provides request-scoped execution context for application services.
package execution

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

// ExecutionContext carries request-scoped context, a journal request ID, and optional complete/fail callbacks.
type ExecutionContext struct {
	context.Context

	requestID        libjournal.RequestID
	callbackComplete journal.CompleteFunc
	callbackFail     journal.FailFunc
}

// NewExecutionContext creates an ExecutionContext with required complete and fail callbacks.
// Returns an error if either callback is nil. A nil parent is replaced with context.Background.
func NewExecutionContext(parent context.Context, requestID libjournal.RequestID, complete journal.CompleteFunc, fail journal.FailFunc) (ExecutionContext, context.CancelFunc, error) {
	if complete == nil || fail == nil {
		return ExecutionContext{}, nil, errors.New("nil callback")
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext{
		Context:          ctx,
		requestID:        requestID,
		callbackComplete: complete,
		callbackFail:     fail,
	}, cancel, nil
}

// NewExecutionContextWithoutCallback creates an ExecutionContext without callbacks.
// Complete and Fail become no-ops. A nil parent is replaced with context.Background.
func NewExecutionContextWithoutCallback(parent context.Context, requestID libjournal.RequestID) (ExecutionContext, context.CancelFunc) {
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

// Fail reports cause through the configured fail callback, if any.
// When no callback is set, Fail returns nil.
func (e ExecutionContext) Fail(cause error) error {
	if e.callbackFail == nil {
		return nil
	}

	return e.callbackFail(e.Context, e.requestID, cause)
}
