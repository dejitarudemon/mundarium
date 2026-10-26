package locations_test

import (
	"context"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"gorm.io/gorm"
)

type completeRecorder struct {
	calls     []libjournal.Result
	failCalls []error
	err       error
	failErr   error
}

func (r *completeRecorder) complete(ctx context.Context, requestID, groupRequestID libjournal.ID, result libjournal.Result, tx *gorm.DB) error {
	r.calls = append(r.calls, result)
	return r.err
}

func (r *completeRecorder) fail(ctx context.Context, requestID, groupRequestID libjournal.ID, cause error, tx *gorm.DB) error {
	r.failCalls = append(r.failCalls, cause)
	return r.failErr
}

func newExecCtx(t *testing.T, rec *completeRecorder) (execution.ExecutionContext, context.CancelFunc) {
	t.Helper()

	requestID := libjournal.ID("test-request")
	groupID := libjournal.ID("test-group")
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
