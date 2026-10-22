package locations_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
)

type completeRecorder struct {
	calls     []journal.Result
	failCalls []error
	err       error
	failErr   error
}

func (r *completeRecorder) complete(ctx context.Context, requestID libjournal.RequestID, result journal.Result) error {
	r.calls = append(r.calls, result)
	return r.err
}

func (r *completeRecorder) fail(ctx context.Context, requestID libjournal.RequestID, cause error) error {
	r.failCalls = append(r.failCalls, cause)
	return r.failErr
}

func newExecCtx(t *testing.T, rec *completeRecorder) (execution.ExecutionContext, context.CancelFunc) {
	t.Helper()

	if rec == nil {
		ctx, cancel := execution.NewExecutionContextWithoutCallback(context.Background(), libjournal.RequestID("test-request"))
		return ctx, cancel
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		libjournal.RequestID("test-request"),
		rec.complete,
		rec.fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func idPtr(id types.ID) *types.ID {
	return &id
}

func seedRoot(t *testing.T, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: nil,
		Type:     types.LocationType_WORLD,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation seed: %v", err)
	}
	return loc
}

func seedChild(t *testing.T, parentID types.ID, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: idPtr(parentID),
		Type:     types.LocationType_REGION,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	return loc
}
