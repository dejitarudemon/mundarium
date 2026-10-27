package locations_test

import (
	"errors"
	"strings"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
)

func TestCreateAndGetConnection(t *testing.T) {
	resetDB(t)
	from := seedRoot(t, "from")
	to := seedRoot(t, "to")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	expires := int64(1_700_000_000)
	out, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
		FromID:        from.ID,
		ToID:          to.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":true}`),
		Details:       repositories.JSON(`{"secret":true}`),
		Description:   "tunnel",
		ExpiresAt:     &expires,
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	if out.ID == 0 || out.FromID != from.ID || out.ToID != to.ID {
		t.Fatalf("unexpected create output: %+v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Create || rec.calls[0].Entity != "LOCATION_CONNECTION" {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	got, err := testSvc.GetConnection(ctx, out.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.Description != "tunnel" || !got.Bidirectional || got.ExpiresAt == nil || *got.ExpiresAt != expires {
		t.Fatalf("unexpected got: %+v", got)
	}
}

func TestCreateConnection_AllowsSelfLink(t *testing.T) {
	resetDB(t)
	loc := seedRoot(t, "loop")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	out, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
		FromID:      loc.ID,
		ToID:        loc.ID,
		Description: "corridor",
	})
	if err != nil {
		t.Fatalf("CreateConnection self-link: %v", err)
	}
	if out.FromID != out.ToID {
		t.Fatalf("want self-link, got %+v", out)
	}
}

func TestCreateConnection_ValidationAndFK(t *testing.T) {
	resetDB(t)
	loc := seedRoot(t, "place")
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	t.Run("zero from", func(t *testing.T) {
		_, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{ToID: loc.ID})
		if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
			t.Fatalf("want InvalidRequest, got %v", err)
		}
	})

	t.Run("missing to location", func(t *testing.T) {
		_, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
			FromID: loc.ID,
			ToID:   999,
		})
		if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
			t.Fatalf("want InvalidRequest for FK, got %v", err)
		}
	})
}

func TestGetConnection_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.GetConnection(ctx, 999)
	if _, ok := errors.AsType[svcerrs.NotFound](err); !ok {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestFindConnections(t *testing.T) {
	resetDB(t)
	a := seedRoot(t, "A")
	b := seedRoot(t, "B")
	c := seedRoot(t, "C")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	ab, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{FromID: a.ID, ToID: b.ID, Description: "ab"})
	if err != nil {
		t.Fatalf("create ab: %v", err)
	}
	_, err = testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{FromID: a.ID, ToID: c.ID, Description: "ac"})
	if err != nil {
		t.Fatalf("create ac: %v", err)
	}
	ba, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
		FromID:        b.ID,
		ToID:          a.ID,
		Bidirectional: true,
		Description:   "ba",
	})
	if err != nil {
		t.Fatalf("create ba: %v", err)
	}

	got, err := testSvc.FindConnections(ctx, svclocations.FindConnectionsInput{FromID: &a.ID, ToID: &b.ID})
	if err != nil {
		t.Fatalf("FindConnections directed: %v", err)
	}
	if len(got.Connections) != 1 || got.Connections[0].ID != ab.ID {
		t.Fatalf("want only ab, got %#v", got.Connections)
	}

	either, err := testSvc.FindConnections(ctx, svclocations.FindConnectionsInput{
		FromID:          &a.ID,
		ToID:            &b.ID,
		EitherDirection: true,
	})
	if err != nil {
		t.Fatalf("FindConnections either: %v", err)
	}
	if len(either.Connections) != 2 {
		t.Fatalf("want ab+ba, got %#v", either.Connections)
	}

	involving, err := testSvc.FindConnections(ctx, svclocations.FindConnectionsInput{LocationID: &b.ID})
	if err != nil {
		t.Fatalf("FindConnections involving: %v", err)
	}
	if len(involving.Connections) != 2 {
		t.Fatalf("want connections involving B, got %#v", involving.Connections)
	}
	_ = ba
}

func TestPatchConnectionFields(t *testing.T) {
	resetDB(t)
	a := seedRoot(t, "A")
	b := seedRoot(t, "B")
	c := seedRoot(t, "C")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	created, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
		FromID:      a.ID,
		ToID:        b.ID,
		Description: "old",
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	rec := &completeRecorder{}
	patchCtx, patchCancel := newExecCtx(t, rec)
	defer patchCancel()

	if err := testSvc.PatchConnectionTo(patchCtx, created.ID, c.ID); err != nil {
		t.Fatalf("PatchConnectionTo: %v", err)
	}
	if err := testSvc.PatchConnectionBidirectional(patchCtx, created.ID, true); err != nil {
		t.Fatalf("PatchConnectionBidirectional: %v", err)
	}
	if err := testSvc.PatchConnectionDescription(patchCtx, created.ID, "new"); err != nil {
		t.Fatalf("PatchConnectionDescription: %v", err)
	}
	if err := testSvc.PatchConnectionState(patchCtx, created.ID, repositories.JSON(`{"locked":false}`)); err != nil {
		t.Fatalf("PatchConnectionState: %v", err)
	}
	if err := testSvc.PatchConnectionDetails(patchCtx, created.ID, repositories.JSON(`{"note":"x"}`)); err != nil {
		t.Fatalf("PatchConnectionDetails: %v", err)
	}
	expires := int64(42)
	if err := testSvc.PatchConnectionExpiresAt(patchCtx, created.ID, &expires); err != nil {
		t.Fatalf("PatchConnectionExpiresAt: %v", err)
	}

	got, err := testSvc.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ToID != c.ID || !got.Bidirectional || got.Description != "new" {
		t.Fatalf("unexpected patched connection: %+v", got)
	}
	if got.ExpiresAt == nil || *got.ExpiresAt != expires {
		t.Fatalf("unexpected ExpiresAt: %#v", got.ExpiresAt)
	}
	if len(rec.calls) != 6 {
		t.Fatalf("want 6 patch completes, got %#v", rec.calls)
	}
	for _, call := range rec.calls {
		if call.Action != journal.Action_Patch || call.Entity != "LOCATION_CONNECTION" {
			t.Fatalf("unexpected patch complete: %#v", call)
		}
	}

	if err := testSvc.PatchConnectionExpiresAt(patchCtx, created.ID, nil); err != nil {
		t.Fatalf("PatchConnectionExpiresAt clear: %v", err)
	}
	got, err = testSvc.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection after clear: %v", err)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("want cleared ExpiresAt, got %#v", got.ExpiresAt)
	}
}

func TestPatchConnection_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	err := testSvc.PatchConnectionDescription(ctx, 999, "x")
	if _, ok := errors.AsType[svcerrs.NotFound](err); !ok {
		t.Fatalf("want NotFound, got %v", err)
	}
}

func TestUpdateConnection(t *testing.T) {
	resetDB(t)
	a := seedRoot(t, "A")
	b := seedRoot(t, "B")
	c := seedRoot(t, "C")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	created, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{
		FromID:      a.ID,
		ToID:        b.ID,
		Description: "old",
		ExpiresAt:   ptrInt64(100),
	})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	rec := &completeRecorder{}
	updateCtx, updateCancel := newExecCtx(t, rec)
	defer updateCancel()

	desc := "updated"
	bidirectional := true
	details := repositories.JSON(`{"v":1}`)
	if err := testSvc.UpdateConnection(updateCtx, svclocations.UpdateConnectionInput{
		ID:            created.ID,
		ToID:          &c.ID,
		Bidirectional: &bidirectional,
		Description:   &desc,
		Details:       &details,
		ClearExpiresAt: true,
	}); err != nil {
		t.Fatalf("UpdateConnection: %v", err)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Update {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}

	got, err := testSvc.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ToID != c.ID || !got.Bidirectional || got.Description != "updated" || got.ExpiresAt != nil {
		t.Fatalf("unexpected updated connection: %+v", got)
	}
	if !strings.Contains(string(got.Details), `"v"`) {
		t.Fatalf("unexpected details: %s", got.Details)
	}
}

func TestUpdateConnection_NoFields(t *testing.T) {
	resetDB(t)
	a := seedRoot(t, "A")
	b := seedRoot(t, "B")
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	created, err := testSvc.CreateConnection(ctx, svclocations.CreateConnectionInput{FromID: a.ID, ToID: b.ID})
	if err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}

	err = testSvc.UpdateConnection(ctx, svclocations.UpdateConnectionInput{ID: created.ID})
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest for empty update, got %v", err)
	}
}

func ptrInt64(v int64) *int64 { return &v }
