package events_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolsevents.UpdateEventLocationTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventImportanceTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventSummaryTool)(nil)
	_ tools.Tool = (*toolsevents.UpdateEventDetailsTool)(nil)
)

func TestUpdateEventFieldTools(t *testing.T) {
	resetDB(t)

	ev, err := testRepo.CreateEvent(context.Background(), &repoevents.Event{
		Importance: 10, CreatedAtGameTime: 1, Summary: "old",
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	locTool, err := toolsevents.NewUpdateEventLocationTool(defPath(t, "update_event_location.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("location tool: %v", err)
	}
	impTool, err := toolsevents.NewUpdateEventImportanceTool(defPath(t, "update_event_importance.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("importance tool: %v", err)
	}
	sumTool, err := toolsevents.NewUpdateEventSummaryTool(defPath(t, "update_event_summary.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("summary tool: %v", err)
	}
	detTool, err := toolsevents.NewUpdateEventDetailsTool(defPath(t, "update_event_details.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("details tool: %v", err)
	}

	for _, tool := range []tools.Tool{locTool, impTool, sumTool, detTool} {
		if !tool.IsIdempotent() {
			t.Fatalf("%s must be idempotent", tool.Definition().Name)
		}
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	locID := repositories.ID(9)
	if _, err := locTool.Execute(ctx, mustJSON(t, map[string]any{"id": ev.ID.String(), "location_id": locID})); err != nil {
		t.Fatalf("location: %v", err)
	}
	if _, err := impTool.Execute(ctx, mustJSON(t, map[string]any{"id": ev.ID.String(), "importance": 55})); err != nil {
		t.Fatalf("importance: %v", err)
	}
	if _, err := sumTool.Execute(ctx, mustJSON(t, map[string]any{"id": ev.ID.String(), "summary": "new"})); err != nil {
		t.Fatalf("summary: %v", err)
	}
	if _, err := detTool.Execute(ctx, mustJSON(t, map[string]any{"id": ev.ID.String(), "details": map[string]any{"k": 1}})); err != nil {
		t.Fatalf("details: %v", err)
	}

	got, err := testRepo.GetEvent(context.Background(), ev.ID)
	if err != nil {
		t.Fatalf("GetEvent: %v", err)
	}
	if got.LocationID == nil || *got.LocationID != locID || got.Importance != 55 || got.Summary != "new" {
		t.Fatalf("unexpected event: %+v", got)
	}

	_, err = locTool.Execute(ctx, mustJSON(t, map[string]any{"id": ev.ID.String(), "location_id": 0}))
	if _, ok := errors.AsType[toolerrs.InvalidRequest](err); !ok {
		t.Fatalf("want InvalidRequest for zero location, got %v", err)
	}
}

func TestUpdateEventFieldTools_NilService(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"location", func() error {
			_, err := toolsevents.NewUpdateEventLocationTool(defPath(t, "update_event_location.json"), nil, nil)
			return err
		}},
		{"importance", func() error {
			_, err := toolsevents.NewUpdateEventImportanceTool(defPath(t, "update_event_importance.json"), nil, nil)
			return err
		}},
		{"summary", func() error {
			_, err := toolsevents.NewUpdateEventSummaryTool(defPath(t, "update_event_summary.json"), nil, nil)
			return err
		}},
		{"details", func() error {
			_, err := toolsevents.NewUpdateEventDetailsTool(defPath(t, "update_event_details.json"), nil, nil)
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.fn(); !errors.Is(err, toolerrs.ErrServiceIsNil) {
				t.Fatalf("want ErrServiceIsNil, got %v", err)
			}
		})
	}
}

func TestUpdateEventLocationTool_LoadsDefinition(t *testing.T) {
	tool, err := toolsevents.NewUpdateEventLocationTool(defPath(t, "update_event_location.json"), testSvc, nil)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if tool.Definition().Name != "update_event_location" {
		t.Fatalf("unexpected name %q", tool.Definition().Name)
	}
}
