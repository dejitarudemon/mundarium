package locations_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
)

var (
	_ tools.Tool = (*toolslocations.UpdateConnectionToTool)(nil)
	_ tools.Tool = (*toolslocations.UpdateConnectionStateTool)(nil)
	_ tools.Tool = (*toolslocations.UpdateConnectionDetailsTool)(nil)
	_ tools.Tool = (*toolslocations.UpdateConnectionDescriptionTool)(nil)
	_ tools.Tool = (*toolslocations.UpdateConnectionExpiresAtTool)(nil)
	_ tools.Tool = (*toolslocations.UpdateConnectionBidirectionalTool)(nil)
)

func TestUpdateConnectionFieldTools_NilService(t *testing.T) {
	cases := []struct {
		name string
		fn   func() error
	}{
		{"to", func() error {
			_, err := toolslocations.NewUpdateConnectionToTool(updateConnectionToDefinitionPath(t), nil, nil)
			return err
		}},
		{"state", func() error {
			_, err := toolslocations.NewUpdateConnectionStateTool(updateConnectionStateDefinitionPath(t), nil, nil)
			return err
		}},
		{"details", func() error {
			_, err := toolslocations.NewUpdateConnectionDetailsTool(updateConnectionDetailsDefinitionPath(t), nil, nil)
			return err
		}},
		{"description", func() error {
			_, err := toolslocations.NewUpdateConnectionDescriptionTool(updateConnectionDescriptionDefinitionPath(t), nil, nil)
			return err
		}},
		{"expires_at", func() error {
			_, err := toolslocations.NewUpdateConnectionExpiresAtTool(updateConnectionExpiresAtDefinitionPath(t), nil, nil)
			return err
		}},
		{"bidirectional", func() error {
			_, err := toolslocations.NewUpdateConnectionBidirectionalTool(updateConnectionBidirectionalDefinitionPath(t), nil, nil)
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

func TestUpdateConnectionFieldTools_LoadsDefinitionAndIdempotent(t *testing.T) {
	cases := []struct {
		name string
		want string
		new  func() (tools.Tool, error)
	}{
		{"to", "update_connection_to", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionToTool(updateConnectionToDefinitionPath(t), testSvc, nil)
		}},
		{"state", "update_connection_state", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionStateTool(updateConnectionStateDefinitionPath(t), testSvc, nil)
		}},
		{"details", "update_connection_details", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDetailsTool(updateConnectionDetailsDefinitionPath(t), testSvc, nil)
		}},
		{"description", "update_connection_description", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDescriptionTool(updateConnectionDescriptionDefinitionPath(t), testSvc, nil)
		}},
		{"expires_at", "update_connection_expires_at", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionExpiresAtTool(updateConnectionExpiresAtDefinitionPath(t), testSvc, nil)
		}},
		{"bidirectional", "update_connection_bidirectional", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionBidirectionalTool(updateConnectionBidirectionalDefinitionPath(t), testSvc, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tool, err := tc.new()
			if err != nil {
				t.Fatalf("new tool: %v", err)
			}
			def := tool.Definition()
			if def.Name != tc.want {
				t.Fatalf("want %q, got %q", tc.want, def.Name)
			}
			def.Name = "mutated"
			if tool.Definition().Name != tc.want {
				t.Fatal("Definition must return a copy")
			}
			if !tool.IsIdempotent() {
				t.Fatal("field patch tools must be idempotent")
			}
		})
	}
}

func TestUpdateConnectionFieldTools_Execute(t *testing.T) {
	resetDB(t)

	a := seedLocation(t, "A")
	b := seedLocation(t, "B")
	c := seedLocation(t, "C")
	conn := seedConnection(t, a.ID, b.ID, "old", false)

	ctx, cancel := newExecCtx(t)
	defer cancel()

	toTool, err := toolslocations.NewUpdateConnectionToTool(updateConnectionToDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionToTool: %v", err)
	}
	if _, err := toTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionToArgs{ID: conn.ID, ToID: c.ID})); err != nil {
		t.Fatalf("update_connection_to: %v", err)
	}

	stateTool, err := toolslocations.NewUpdateConnectionStateTool(updateConnectionStateDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionStateTool: %v", err)
	}
	state := repositories.JSON(`{"open":true}`)
	if _, err := stateTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionStateArgs{ID: conn.ID, State: state})); err != nil {
		t.Fatalf("update_connection_state: %v", err)
	}

	detailsTool, err := toolslocations.NewUpdateConnectionDetailsTool(updateConnectionDetailsDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionDetailsTool: %v", err)
	}
	details := repositories.JSON(`{"hidden":true}`)
	if _, err := detailsTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionDetailsArgs{ID: conn.ID, Details: details})); err != nil {
		t.Fatalf("update_connection_details: %v", err)
	}

	descTool, err := toolslocations.NewUpdateConnectionDescriptionTool(updateConnectionDescriptionDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionDescriptionTool: %v", err)
	}
	if _, err := descTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionDescriptionArgs{ID: conn.ID, Description: "new"})); err != nil {
		t.Fatalf("update_connection_description: %v", err)
	}

	expiresTool, err := toolslocations.NewUpdateConnectionExpiresAtTool(updateConnectionExpiresAtDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionExpiresAtTool: %v", err)
	}
	expires := int64(99)
	if _, err := expiresTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionExpiresAtArgs{ID: conn.ID, ExpiresAt: &expires})); err != nil {
		t.Fatalf("update_connection_expires_at set: %v", err)
	}

	biTool, err := toolslocations.NewUpdateConnectionBidirectionalTool(updateConnectionBidirectionalDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionBidirectionalTool: %v", err)
	}
	if _, err := biTool.Execute(ctx, mustJSON(t, toolslocations.UpdateConnectionBidirectionalArgs{ID: conn.ID, Bidirectional: true})); err != nil {
		t.Fatalf("update_connection_bidirectional: %v", err)
	}

	got, err := testRepo.GetConnection(context.Background(), conn.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	if got.ToID != c.ID || !got.Bidirectional || got.Description != "new" {
		t.Fatalf("unexpected connection after patches: %+v", got)
	}
	if got.ExpiresAt == nil || *got.ExpiresAt != expires {
		t.Fatalf("unexpected ExpiresAt: %#v", got.ExpiresAt)
	}

	if _, err := expiresTool.Execute(ctx, mustJSON(t, map[string]any{"id": conn.ID, "expires_at": nil})); err != nil {
		t.Fatalf("update_connection_expires_at clear: %v", err)
	}
	got, err = testRepo.GetConnection(context.Background(), conn.ID)
	if err != nil {
		t.Fatalf("GetConnection after clear: %v", err)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("want cleared ExpiresAt, got %#v", got.ExpiresAt)
	}
}

func TestUpdateConnectionToTool_ZeroIDs(t *testing.T) {
	tool, err := toolslocations.NewUpdateConnectionToTool(updateConnectionToDefinitionPath(t), testSvc, nil)
	if err != nil {
		t.Fatalf("NewUpdateConnectionToTool: %v", err)
	}

	ctx, cancel := newExecCtx(t)
	defer cancel()

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":0,"to_id":1}`))
	if !errors.Is(err, toolerrs.ErrConnectionIDZero) {
		t.Fatalf("want ErrConnectionIDZero, got %v", err)
	}

	_, err = tool.Execute(ctx, json.RawMessage(`{"id":1,"to_id":0}`))
	if !errors.Is(err, toolerrs.ErrToIDZero) {
		t.Fatalf("want ErrToIDZero, got %v", err)
	}
}
