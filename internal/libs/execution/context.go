// Package execution provides request-scoped execution context for application services.
package execution

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libprovider "github.com/dejitarudemon/mundarium/internal/libs/providers"
	"gorm.io/gorm"
)

// ExecutionContext carries request-scoped context for a tool call:
// call/invoke journal IDs, caller role, and optional complete/fail callbacks.
type ExecutionContext struct {
	context.Context

	callID           journal.ID
	invokeID         journal.ID
	role             libprovider.Role
	callbackComplete journal.CompleteFunc
	callbackFail     journal.FailFunc
}

// NewExecutionContext creates an ExecutionContext with required complete and fail callbacks.
// callID is the journal request id; invokeID is the invoke/group id; role is the caller role.
// Returns an error if either callback is nil. A nil parent is replaced with context.Background.
func NewExecutionContext(parent context.Context, callID, invokeID journal.ID, role libprovider.Role, complete journal.CompleteFunc, fail journal.FailFunc) (ExecutionContext, context.CancelFunc, error) {
	if complete == nil || fail == nil {
		return ExecutionContext{}, nil, errors.New("nil callback")
	}
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext{
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
func NewExecutionContextWithoutCallback(parent context.Context, callID, invokeID journal.ID, role libprovider.Role) (ExecutionContext, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}

	ctx, cancel := context.WithCancel(parent)

	return ExecutionContext{
		Context:  ctx,
		callID:   callID,
		invokeID: invokeID,
		role:     role,
	}, cancel
}

// Complete reports result through the configured callback, if any.
// When no callback is set, Complete returns nil.
func (e ExecutionContext) Complete(result journal.Result, tx *gorm.DB) error {
	if e.callbackComplete == nil {
		return nil
	}

	return e.callbackComplete(e.Context, e.callID, e.invokeID, result, tx)
}

// Fail reports cause through the configured fail callback, if any.
// When no callback is set, Fail returns nil.
func (e ExecutionContext) Fail(cause error, tx *gorm.DB) error {
	if e.callbackFail == nil {
		return nil
	}

	return e.callbackFail(e.Context, e.callID, e.invokeID, cause, tx)
}

// InvokeID returns the invoke / conversation group id for this execution.
func (e ExecutionContext) InvokeID() journal.ID {
	return e.invokeID
}

// Role returns the caller role for this execution.
func (e ExecutionContext) Role() libprovider.Role {
	return e.role
}
