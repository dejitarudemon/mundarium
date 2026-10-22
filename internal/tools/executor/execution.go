package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	libtools "github.com/dejitarudemon/mundarium/internal/libs/tools"
	"github.com/dejitarudemon/mundarium/internal/tools"
)

// Execute runs call against a registered tool.
// Non-idempotent tools are journaled by request id derived from invokeID and call.ID.
func (e *Executor) Execute(ctx context.Context, invokeID string, call libtools.Call) (json.RawMessage, error) {
	tool, ok := e.tools[call.Name]
	if !ok {
		return nil, ErrToolNotFound
	}

	requestID := createRequestID(invokeID, call.ID)

	if tool.IsIdempotent() {
		return e.executeIdempotent(ctx, tool, requestID, call.Args)
	}

	return e.executeNotIdempotent(ctx, tool, requestID, call.Args)
}

func (e *Executor) executeNotIdempotent(parent context.Context, tool tools.Tool, requestID libjournal.RequestID, args json.RawMessage) (json.RawMessage, error) {
	toolName := tool.Definition().Name
	argsHash := hashArgs(args)

	complete, fail, err := e.journal.Register(parent, requestID, toolName, argsHash)
	if err != nil {
		if _, ok := errors.AsType[journal.AlreadyRegistered](err); ok {
			return e.resolveAlreadyRegistered(parent, toolName, requestID, argsHash)
		}
		return nil, err
	}

	ctx, cancel, err := execution.NewExecutionContext(parent, requestID, complete, fail)
	if err != nil {
		return nil, err
	}
	defer cancel()

	return tool.Execute(ctx, args)
}

func (e *Executor) resolveAlreadyRegistered(ctx context.Context, toolName string, requestID libjournal.RequestID, argsHash []byte) (json.RawMessage, error) {
	req, err := e.journal.Find(ctx, requestID)
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
		if req.Error != nil {
			return nil, req.Error
		}
		return nil, ErrRequestAlreadyFailed
	default:
		return nil, ErrUnexpectedRequestStatus
	}
}

func (e *Executor) executeIdempotent(parent context.Context, tool tools.Tool, requestID libjournal.RequestID, args json.RawMessage) (json.RawMessage, error) {
	ctx, cancel := execution.NewExecutionContextWithoutCallback(parent, requestID)
	defer cancel()

	return tool.Execute(ctx, args)
}
