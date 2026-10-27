// Package execution provides a request-scoped ExecutionContext for tools and services.
// Call/invoke identity and caller role use libs/types; completion payloads are generic.
package execution

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

// ExecutionContext carries request-scoped context for a tool call:
// call/invoke IDs, caller role, and optional complete/fail callbacks.
// T is the successful completion payload type (for example journal.Result).
type ExecutionContext[T any] struct {
	context.Context

	callID           types.ID
	invokeID         types.ID
	role             types.Role
	callbackComplete CompleteFunc[T]
	callbackFail     FailFunc
}

// NewExecutionContext creates an ExecutionContext with required complete and fail callbacks.
// callID is the journal request id; invokeID is the invoke/group id; role is the caller role.
// Returns an error if either callback is nil. A nil parent is replaced with context.Background.
func NewExecutionContext[T any](parent context.Context, callID, invokeID types.ID, role types.Role, complete CompleteFunc[T], fail FailFunc) (ExecutionContext[T], context.CancelFunc, error) {
	if complete == nil || fail == nil {
		return ExecutionContext[T]{}, nil, errors.New("nil callback")
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext[T]{
		Context:          ctx,
		callID:           callID,
		invokeID:         invokeID,
		callbackComplete: complete,
		callbackFail:     fail,
		role:             role,
	}, cancel, nil
}

// NewExecutionContextWithoutCallback creates an ExecutionContext without callbacks.
// callID, invokeID, and role are still stored for InvokeID/Role accessors.
// Complete and Fail become no-ops. A nil parent is replaced with context.Background.
func NewExecutionContextWithoutCallback[T any](parent context.Context, callID, invokeID types.ID, role types.Role) (ExecutionContext[T], context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext[T]{
		Context:  ctx,
		callID:   callID,
		invokeID: invokeID,
		role:     role,
	}, cancel
}

// Complete reports result through the configured callback, if any.
// When no callback is set, Complete returns nil.
func (e ExecutionContext[T]) Complete(result T, tx *gorm.DB) error {
	if e.callbackComplete == nil {
		return nil
	}

	return e.callbackComplete(e.Context, e.callID, e.invokeID, result, tx)
}

// Fail reports cause through the configured fail callback, if any.
// When no callback is set, Fail returns nil.
func (e ExecutionContext[T]) Fail(cause error, tx *gorm.DB) error {
	if e.callbackFail == nil {
		return nil
	}

	return e.callbackFail(e.Context, e.callID, e.invokeID, cause, tx)
}

// InvokeID returns the invoke / conversation group id for this execution.
func (e ExecutionContext[T]) InvokeID() types.ID {
	return e.invokeID
}

// Role returns the caller role for this execution.
func (e ExecutionContext[T]) Role() types.Role {
	return e.role
}
