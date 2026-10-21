package locations_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/services"
	"github.com/google/uuid"
)

type completeRecorder struct {
	calls []services.Result
	err   error
}

func (r *completeRecorder) callback(ctx context.Context, requestID journal.RequestID, result services.Result) error {
	r.calls = append(r.calls, result)
	return r.err
}

func newExecCtx(t *testing.T, rec *completeRecorder) (services.ExecutionContext, context.CancelFunc) {
	t.Helper()

	if rec == nil {
		ctx, cancel := services.NewExecutionContextWithoutCallback(context.Background(), journal.RequestID(uuid.New()))
		return ctx, cancel
	}

	ctx, cancel, err := services.NewExecutionContext(
		context.Background(),
		journal.RequestID(uuid.New()),
		rec.callback,
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
