package locations_test

import (
	"context"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"github.com/dejitarudemon/mundarium/internal/libs/providers"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
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

func idPtr(id repositories.ID) *repositories.ID {
	return &id
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
		ParentID: idPtr(parentID),
		Type:     repolocations.LocationType_REGION,
		Name:     name,
	})
	if err != nil {
		t.Fatalf("CreateLocation child: %v", err)
	}
	return loc
}
