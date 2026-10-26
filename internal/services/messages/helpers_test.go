package messages_test

import (
	"context"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	"github.com/dejitarudemon/mundarium/internal/providers"
	"gorm.io/gorm"
)

type completeRecorder struct {
	calls     []journal.Result
	failCalls []error
	err       error
	failErr   error
}

func (r *completeRecorder) complete(ctx context.Context, requestID, groupRequestID journal.ID, result journal.Result, tx *gorm.DB) error {
	r.calls = append(r.calls, result)
	return r.err
}

func (r *completeRecorder) fail(ctx context.Context, requestID, groupRequestID journal.ID, cause error, tx *gorm.DB) error {
	r.failCalls = append(r.failCalls, cause)
	return r.failErr
}

func newExecCtx(t *testing.T, rec *completeRecorder) (execution.ExecutionContext, context.CancelFunc) {
	t.Helper()

	requestID := journal.ID("test-request")
	groupID := journal.ID("test-group")
	role := providers.Role_Master

	if rec == nil {
		ctx, cancel := execution.NewExecutionContextWithoutCallback(context.Background(), requestID, groupID, role)
		return ctx, cancel
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		requestID,
		groupID,
		role,
		rec.complete,
		rec.fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func seedMessage(t *testing.T, invokeID, text string) *repomessages.Message {
	t.Helper()

	msg, err := testRepo.CreateMessage(context.Background(), &repomessages.Message{
		InvokeID: invokeID,
		Text:     text,
		From:     providers.Role_Master,
	})
	if err != nil {
		t.Fatalf("CreateMessage seed: %v", err)
	}
	return msg
}
