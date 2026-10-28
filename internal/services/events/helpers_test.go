package events_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
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

func newExecCtx(t *testing.T, rec *completeRecorder) (execution.ExecutionContext[journal.Result], context.CancelFunc) {
	t.Helper()

	requestID := journal.ID("test-request")
	groupID := journal.ID("test-group")
	role := types.Role_Master

	if rec == nil {
		ctx, cancel := execution.NewExecutionContextWithoutCallback[journal.Result](context.Background(), requestID, groupID, role)
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

func createEvent(t *testing.T, summary string, importance int8, gameTime repositories.Ticks) svcevents.CreateEventOutput {
	t.Helper()
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	out, err := testSvc.CreateEvent(ctx, svcevents.CreateEventInput{
		Importance:        importance,
		CreatedAtGameTime: gameTime,
		Summary:           summary,
		Tags:              []string{"test"},
	})
	if err != nil {
		t.Fatalf("CreateEvent(%q): %v", summary, err)
	}
	return out
}

func newEntityID() entities.ID {
	return entities.New()
}
