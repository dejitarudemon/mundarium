package locations_test

import (
	"context"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
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

func seedRoot(t *testing.T, name string) *repolocations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation seed: %v", err)
	}
	return loc
}

func seedChild(t *testing.T, parentID repositories.ID, name string) *repolocations.Location {
	t.Helper()

	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(parentID),
		Type:     repolocations.LocationType_REGION,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	return loc
}
