package locations_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/google/uuid"
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
		ctx, cancel := execution.NewExecutionContextWithoutCallback(context.Background(), libjournal.RequestID(uuid.New()))
		return ctx, cancel
	}

	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		libjournal.RequestID(uuid.New()),
		rec.complete,
		rec.fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func seedRoot(t *testing.T, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: 0,
		Type:     types.LocationType_WORLD,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation seed: %v", err)
	}
	if err := testRepo.PatchLocationParent(context.Background(), loc.ID, loc.ID); err != nil {
		t.Fatalf("make root: %v", err)
	}
	loc.ParentID = loc.ID
	return loc
}

func seedChild(t *testing.T, parentID types.ID, name string) *locations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &locations.Location{
		ParentID: parentID,
		Type:     types.LocationType_REGION,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	return loc
}
