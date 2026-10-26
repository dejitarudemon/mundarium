package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

// Execute runs call against a registered tool.
// Non-idempotent tools are journaled under group invokeID and request call.ID.
// The tool's ExecutionContext receives invokeID, call.ID, and call.Role.
func (e *Executor) Execute(ctx context.Context, invokeID string, call tools.Call) (json.RawMessage, error) {
	tool, err := e.lookup(call.Name, call.Role)
	if err != nil {
		e.error("failed to execute tool", "type", "Executor", "func", "Execute", "tool", call.Name, "role", call.Role, "invoke_id", invokeID, "call_id", call.ID, "error", err)
		return nil, err
	}

	if tool.IsIdempotent() {
		raw, err := e.executeIdempotent(ctx, tool, invokeID, call.ID, call.Role, call.Args)
		if err != nil {
			e.error("failed to execute tool", "type", "Executor", "func", "Execute", "tool", call.Name, "role", call.Role, "invoke_id", invokeID, "call_id", call.ID, "error", err)
			return nil, err
		}
		e.debug("tool is executed", "type", "Executor", "func", "Execute", "tool", call.Name, "role", call.Role, "invoke_id", invokeID, "call_id", call.ID, "idempotent", true)
		return raw, nil
	}

	raw, err := e.executeNotIdempotent(ctx, tool, invokeID, call.ID, call.Role, call.Args)
	if err != nil {
		e.error("failed to execute tool", "type", "Executor", "func", "Execute", "tool", call.Name, "role", call.Role, "invoke_id", invokeID, "call_id", call.ID, "error", err)
		return nil, err
	}
	e.info("tool is executed", "type", "Executor", "func", "Execute", "tool", call.Name, "role", call.Role, "invoke_id", invokeID, "call_id", call.ID, "idempotent", false)
	return raw, nil
}

// lookup resolves a registered tool and checks role permission under the read lock.
func (e *Executor) lookup(name string, role types.Role) (tools.Tool, error) {
	e.mx.RLock()
	defer e.mx.RUnlock()

	tool, ok := e.tools[name]
	if !ok {
		return nil, toolerrs.NewToolNotFound()
	}
	if err := e.checkPermission(role, tool.Definition()); err != nil {
		return nil, err
	}
	return tool, nil
}

func (e *Executor) executeNotIdempotent(parent context.Context, tool tools.Tool, invokeID, callID string, role types.Role, args json.RawMessage) (json.RawMessage, error) {
	toolName := tool.Definition().Name
	argsHash := hashArgs(args)

	complete, fail, err := e.journal.Register(parent, toJournalID(callID), toJournalID(invokeID), toolName, argsHash)
	if err != nil {
		if _, ok := errors.AsType[journal.AlreadyRegistered](err); ok {
			return e.resolveAlreadyRegistered(parent, toolName, callID, invokeID, argsHash)
		}
		return nil, toolerrs.From(err)
	}

	ctx, cancel, err := execution.NewExecutionContext(parent, toJournalID(callID), toJournalID(invokeID), role, complete, fail)
	if err != nil {
		return nil, toolerrs.From(err)
	}
	defer cancel()

	return tool.Execute(ctx, args)
}

func (e *Executor) resolveAlreadyRegistered(ctx context.Context, toolName string, callID, invokeID string, argsHash []byte) (json.RawMessage, error) {
	req, err := e.journal.Find(ctx, toJournalID(callID), toJournalID(invokeID))
	if err != nil {
		return nil, toolerrs.From(err)
	}

	if req.Tool != toolName {
		return nil, toolerrs.NewRequestToolMismatch()
	}
	if !bytes.Equal(req.ArgsHash, argsHash) {
		return nil, toolerrs.NewRequestArgsMismatch()
	}

	switch req.Status {
	case journal.Status_Completed:
		raw, err := resultPayload(req.Result)
		if err != nil {
			return nil, toolerrs.NewInvalidRequest(err)
		}
		e.debug("tool result replayed", "type", "Executor", "func", "resolveAlreadyRegistered", "tool", toolName, "invoke_id", invokeID, "call_id", callID)
		return raw, nil
	case journal.Status_Running:
		return nil, toolerrs.NewRequestStillRunning()
	case journal.Status_Failed:
		if err := req.Error.Err(); err != nil {
			return nil, toolerrs.From(err)
		}
		return nil, toolerrs.NewRequestAlreadyFailed()
	default:
		return nil, toolerrs.NewUnexpectedRequestStatus()
	}
}

func (e *Executor) executeIdempotent(parent context.Context, tool tools.Tool, invokeID, callID string, role types.Role, args json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := execution.NewExecutionContextWithoutCallback[journal.Result](parent, toJournalID(callID), toJournalID(invokeID), role)
	defer cancel()

	return tool.Execute(ctx, args)
}

func (e *Executor) checkPermission(role types.Role, def definition.Definition) error {
	defs, ok := e.allowed[role]
	if !ok {
		return toolerrs.NewPermissionDenied()
	}

	if !slices.ContainsFunc(defs, func(d definition.Definition) bool {
		return d.Name == def.Name
	}) {
		return toolerrs.NewPermissionDenied()
	}

	return nil
}
