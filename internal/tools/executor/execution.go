package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	libtools "github.com/dejitarudemon/mundarium/internal/libs/tools"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
)

// Execute runs call against a registered tool.
// Non-idempotent tools are journaled under group invokeID and request call.ID.
func (e *Executor) Execute(ctx context.Context, invokeID string, call libtools.Call) (json.RawMessage, error) {
	tool, err := e.lookup(call.Name, call.Role)
	if err != nil {
		return nil, err
	}

	if tool.IsIdempotent() {
		return e.executeIdempotent(ctx, tool, invokeID, call.ID, call.Args)
	}

	return e.executeNotIdempotent(ctx, tool, invokeID, call.ID, call.Args)
}

// lookup resolves a registered tool and checks role permission under the read lock.
func (e *Executor) lookup(name string, role providers.Role) (tools.Tool, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()

	tool, ok := e.tools[name]
	if !ok {
		return nil, ErrToolNotFound
	}
	if err := e.checkPermission(role, tool.Definition()); err != nil {
		return nil, err
	}
	return tool, nil
}

func (e *Executor) executeNotIdempotent(parent context.Context, tool tools.Tool, invokeID, callID string, args json.RawMessage) (json.RawMessage, error) {
	toolName := tool.Definition().Name
	argsHash := hashArgs(args)

	complete, fail, err := e.journal.Register(parent, toJournalID(callID), toJournalID(invokeID), toolName, argsHash)
	if err != nil {
		if _, ok := errors.AsType[journal.AlreadyRegistered](err); ok {
			return e.resolveAlreadyRegistered(parent, toolName, callID, invokeID, argsHash)
		}
		return nil, err
	}

	ctx, cancel, err := execution.NewExecutionContext(parent, toJournalID(callID), toJournalID(invokeID), complete, fail)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return tool.Execute(ctx, args)
}

func (e *Executor) resolveAlreadyRegistered(ctx context.Context, toolName string, callID, invokeID string, argsHash []byte) (json.RawMessage, error) {
	req, err := e.journal.Find(ctx, toJournalID(callID), toJournalID(invokeID))
	if err != nil {
		return nil, err
	}

	if req.Tool != toolName {
		return nil, ErrRequestToolMismatch
	}
	if !bytes.Equal(req.ArgsHash, argsHash) {
		return nil, ErrRequestArgsMismatch
	}

	switch req.Status {
	case libjournal.Status_Completed:
		return resultPayload(req.Result)
	case libjournal.Status_Running:
		return nil, ErrRequestStillRunning
	case libjournal.Status_Failed:
		if err := req.Error.Err(); err != nil {
			return nil, err
		}
		return nil, ErrRequestAlreadyFailed
	default:
		return nil, ErrUnexpectedRequestStatus
	}
}

func (e *Executor) executeIdempotent(parent context.Context, tool tools.Tool, invokeID, callID string, args json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := execution.NewExecutionContextWithoutCallback(parent, toJournalID(callID), toJournalID(invokeID))
	defer cancel()

	return tool.Execute(ctx, args)
}

func (e *Executor) checkPermission(role providers.Role, def definition.Definition) error {
	defs, ok := e.allowed[role]
	if !ok {
		return ErrPermissionDenied
	}

	if !slices.ContainsFunc(defs, func(d definition.Definition) bool {
		return d.Name == def.Name
	}) {
		return ErrPermissionDenied
	}

	return nil
}
