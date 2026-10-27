package locations_test

import (
	"errors"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
)

func TestFindLocations(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "World")
	north := seedChild(t, root.ID, "North")
	_ = seedChild(t, north.ID, "North Gate")
	_ = seedChild(t, root.ID, "South")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
		ScopeID: root.ID,
		Query:   new("Gate"),
		Depth:   new(2),
	})
	if err != nil {
		t.Fatalf("FindLocations: %v", err)
	}
	if out.Scope.Name != "World" || len(out.Children[2]) != 1 || out.Children[2][0].Name != "North Gate" || out.HasMore != 0 {
		t.Fatalf("want World scope + North Gate at depth 2, got %#v", out)
	}
	if len(rec.calls) != 1 || rec.calls[0].Action != journal.Action_Get {
		t.Fatalf("unexpected complete calls: %#v", rec.calls)
	}
}

func TestFindLocations_HasMore(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "World")
	_ = seedChild(t, root.ID, "A")
	_ = seedChild(t, root.ID, "B")
	_ = seedChild(t, root.ID, "C")

	ctx, cancel := newExecCtx(t, &completeRecorder{})
	defer cancel()

	out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
		ScopeID: root.ID,
		Depth:   new(1),
		Limit:   new(2),
	})
	if err != nil {
		t.Fatalf("FindLocations: %v", err)
	}
	// 3 children, limit 2 => 2 returned, has_more = 1
	if out.Scope.Name != "World" || len(out.Children[1]) != 2 || out.HasMore != 1 {
		t.Fatalf("want scope + 2 children and has_more=1, got %#v", out)
	}
}

func TestFindLocations_ValidationError(t *testing.T) {
	resetDB(t)
	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	_, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{ScopeID: 0})
	if _, ok := errors.AsType[svcerrs.InvalidRequest](err); !ok {
		t.Fatalf("want errs.InvalidRequest for scope id, got %v", err)
	}
	if len(rec.failCalls) == 0 {
		t.Fatal("want Fail called")
	}
}

func TestFindChildLocations(t *testing.T) {
	resetDB(t)
	root := seedRoot(t, "root")
	a := seedChild(t, root.ID, "A")
	_ = seedChild(t, root.ID, "B")
	_ = seedChild(t, a.ID, "A1")

	rec := &completeRecorder{}
	ctx, cancel := newExecCtx(t, rec)
	defer cancel()

	depth0, err := testSvc.FindChildLocations(ctx, root.ID, 0)
	if err != nil {
		t.Fatalf("depth 0: %v", err)
	}
	if len(depth0) != 0 {
		t.Fatalf("depth 0 want empty, got %#v", depth0)
	}

	depth1, err := testSvc.FindChildLocations(ctx, root.ID, 1)
	if err != nil {
		t.Fatalf("depth 1: %v", err)
	}
	if len(depth1[1]) != 2 {
		t.Fatalf("depth 1 want 2, got %#v", depth1)
	}

	depth2, err := testSvc.FindChildLocations(ctx, root.ID, 2)
	if err != nil {
		t.Fatalf("depth 2: %v", err)
	}
	if len(depth2[1]) != 2 || len(depth2[2]) != 1 {
		t.Fatalf("depth 2 want 2+1, got %#v", depth2)
	}
	if depth2[2][0].Name != "A1" {
		t.Fatalf("want grandchild A1, got %+v", depth2[2][0])
	}
	if len(rec.calls) < 1 {
		t.Fatal("expected Complete to be called")
	}
}

func TestFindChildLocations_NotFound(t *testing.T) {
	resetDB(t)
	ctx, cancel := newExecCtx(t, nil)
	defer cancel()

	_, err := testSvc.FindChildLocations(ctx, 999, 1)
	if _, ok := errors.AsType[svcerrs.NotFound](err); !ok {
		t.Fatalf("expected errs.NotFound, got %v", err)
	}
}

// namesAt returns location names at the given relative depth, in response order.
func namesAt(out svclocations.FindLocationsOutput, depth int) []string {
	locs := out.Children[depth]
	names := make([]string, len(locs))
	for i, loc := range locs {
		names[i] = loc.Name
	}
	return names
}

func childTotal(out svclocations.FindLocationsOutput) int {
	n := 0
	for _, locs := range out.Children {
		n += len(locs)
	}
	return n
}

// TestCreateAndFindLocations_ThreeLevelChain builds World → North → North Gate via CreateLocation,
// plus a South sibling under World, then asserts FindLocations behavior for each filter.
//
// Tree:
//
//	World (world)
//	├── North (region)
//	│   └── North Gate (site, description "stone arch")
//	└── South (region)
func TestCreateAndFindLocations_ThreeLevelChain(t *testing.T) {
	resetDB(t)

	ctx, cancel := newExecCtx(t, &completeRecorder{})
	defer cancel()

	world, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID: nil,
		Type:     "world",
		Name:     "World",
	})
	if err != nil {
		t.Fatalf("create World: %v", err)
	}

	north, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID:    new(world.ID),
		Type:        "region",
		Name:        "North",
		Description: "cold marches",
	})
	if err != nil {
		t.Fatalf("create North: %v", err)
	}
	if north.ParentID == nil || *north.ParentID != world.ID {
		t.Fatalf("North parent: want %d, got %+v", world.ID, north.ParentID)
	}

	gate, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID:    new(north.ID),
		Type:        "site",
		Name:        "North Gate",
		Description: "stone arch",
	})
	if err != nil {
		t.Fatalf("create North Gate: %v", err)
	}
	if gate.ParentID == nil || *gate.ParentID != north.ID {
		t.Fatalf("North Gate parent: want %d, got %+v", north.ID, gate.ParentID)
	}

	south, err := testSvc.CreateLocation(ctx, svclocations.CreateLocationInput{
		ParentID: new(world.ID),
		Type:     "region",
		Name:     "South",
	})
	if err != nil {
		t.Fatalf("create South: %v", err)
	}

	t.Run("depth0_only_scope", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Depth:   new(0),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID || childTotal(out) != 0 || out.HasMore != 0 {
			t.Fatalf("want only World, got %#v", out)
		}
	})

	t.Run("depth1_direct_children", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Depth:   new(1),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID || out.HasMore != 0 {
			t.Fatalf("unexpected scope/has_more: %#v", out)
		}
		got := namesAt(out, 1)
		want := []string{"North", "South"}
		if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
			t.Fatalf("depth 1: want %v, got %v (full %#v)", want, got, out)
		}
		if len(out.Children[2]) != 0 {
			t.Fatalf("depth 1 must not include grandchildren: %#v", out.Children[2])
		}
	})

	t.Run("depth2_all_descendants", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if namesAt(out, 1)[0] != "North" || namesAt(out, 1)[1] != "South" {
			t.Fatalf("depth 1: want North,South got %v", namesAt(out, 1))
		}
		if len(namesAt(out, 2)) != 1 || namesAt(out, 2)[0] != "North Gate" {
			t.Fatalf("depth 2: want [North Gate], got %v", namesAt(out, 2))
		}
		if out.HasMore != 0 {
			t.Fatalf("want has_more=0, got %d", out.HasMore)
		}
	})

	t.Run("query_gate_skips_nonmatching_intermediate", func(t *testing.T) {
		// North does not match "Gate", but traversal must still reach North Gate.
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Query:   new("Gate"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID {
			t.Fatalf("scope must always be World, got %#v", out.Scope)
		}
		if childTotal(out) != 1 || len(out.Children[2]) != 1 || out.Children[2][0].ID != gate.ID {
			t.Fatalf("want only North Gate under children, got %#v", out)
		}
		if len(out.Children[1]) != 0 {
			t.Fatalf("North must not appear (query filter), got %#v", out.Children[1])
		}
	})

	t.Run("query_gate_depth1_misses_grandchild", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Query:   new("Gate"),
			Depth:   new(1),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID || childTotal(out) != 0 {
			t.Fatalf("Gate is at depth 2; depth 1 must yield no children, got %#v", out)
		}
	})

	t.Run("query_description_substring", func(t *testing.T) {
		// "stone" matches Gate description; avoid "arch" — it is also a substring of North's "marches".
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Query:   new("STONE"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if childTotal(out) != 1 || out.Children[2][0].ID != gate.ID {
			t.Fatalf("want North Gate by description, got %#v", out)
		}
	})

	t.Run("query_north_matches_region_and_gate", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Query:   new("North"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if len(namesAt(out, 1)) != 1 || namesAt(out, 1)[0] != "North" {
			t.Fatalf("depth 1: want [North], got %v", namesAt(out, 1))
		}
		if len(namesAt(out, 2)) != 1 || namesAt(out, 2)[0] != "North Gate" {
			t.Fatalf("depth 2: want [North Gate], got %v", namesAt(out, 2))
		}
		if len(out.Children[1]) > 0 {
			for _, loc := range out.Children[1] {
				if loc.ID == south.ID {
					t.Fatalf("South must not match query North: %#v", out)
				}
			}
		}
	})

	t.Run("type_site", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Type:    new("site"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.Type != "world" {
			t.Fatalf("scope type must stay world (unfiltered), got %q", out.Scope.Type)
		}
		if childTotal(out) != 1 || out.Children[2][0].ID != gate.ID {
			t.Fatalf("want only North Gate (site), got %#v", out)
		}
	})

	t.Run("type_region", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Type:    new("region"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		got := namesAt(out, 1)
		if len(got) != 2 || got[0] != "North" || got[1] != "South" {
			t.Fatalf("want North,South regions, got %v", got)
		}
		if len(out.Children[2]) != 0 {
			t.Fatalf("site must not appear for type=region: %#v", out.Children[2])
		}
	})

	t.Run("scope_north_depth1", func(t *testing.T) {
		// Searching from the middle node: Gate is a direct child (relative depth 1).
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: north.ID,
			Depth:   new(1),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != north.ID || len(namesAt(out, 1)) != 1 || namesAt(out, 1)[0] != "North Gate" {
			t.Fatalf("want North scope + North Gate, got %#v", out)
		}
	})

	t.Run("limit_truncates_children_counts_omitted", func(t *testing.T) {
		// depth 2 without filters: North, South, North Gate (3 descendants). limit 1 → has_more=2.
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Depth:   new(2),
			Limit:   new(1),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID {
			t.Fatalf("scope must remain, got %#v", out.Scope)
		}
		if childTotal(out) != 1 || out.HasMore != 2 {
			t.Fatalf("want 1 child and has_more=2, got children=%d has_more=%d full=%#v",
				childTotal(out), out.HasMore, out)
		}
		// BFS order: depth 1 before depth 2, so the single kept child is North.
		if len(out.Children[1]) != 1 || out.Children[1][0].Name != "North" {
			t.Fatalf("limit keeps first by depth then id: want North, got %#v", out)
		}
	})

	t.Run("scope_always_present_even_if_filters_exclude_all", func(t *testing.T) {
		out, err := testSvc.FindLocations(ctx, svclocations.FindLocationsInput{
			ScopeID: world.ID,
			Query:   new("no-such-place"),
			Depth:   new(2),
		})
		if err != nil {
			t.Fatalf("FindLocations: %v", err)
		}
		if out.Scope.ID != world.ID || childTotal(out) != 0 || out.HasMore != 0 {
			t.Fatalf("want World with empty children, got %#v", out)
		}
	})
}
