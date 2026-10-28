package connection_targeted_updates_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// A05: deterministic Executor targeted connection patches — Postgres only, no LLM.
	_ = testutil.LoadDotEnvFromRepo()

	ctx := context.Background()
	pg, conn, err := testutil.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	container = pg

	code := 1
	defer func() {
		_ = container.Terminate(ctx)
		os.Exit(code)
	}()

	log := implemented.New(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	generator := func(db *gorm.DB) *repolocations.LocationRepository {
		repo, err := repolocations.NewLocationRepository(db, 5, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := svclocations.NewService(generator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new locations service: %v\n", err)
		return
	}

	j, err := pgjournal.NewPostgresJournal(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new postgres journal: %v\n", err)
		return
	}

	exec, err := executor.NewExecutor(j, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new executor: %v\n", err)
		return
	}

	root := testutil.RepoRoot()
	role := []types.Role{types.Role_Master}

	toolsToRegister := []struct {
		name string
		make func() (tools.Tool, error)
	}{
		{"update_connection_state", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionStateTool(filepath.Join(root, "definitions", "update_connection_state.json"), svc, log)
		}},
		{"update_connection_details", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDetailsTool(filepath.Join(root, "definitions", "update_connection_details.json"), svc, log)
		}},
		{"update_connection_description", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDescriptionTool(filepath.Join(root, "definitions", "update_connection_description.json"), svc, log)
		}},
		{"update_connection_bidirectional", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionBidirectionalTool(filepath.Join(root, "definitions", "update_connection_bidirectional.json"), svc, log)
		}},
		{"update_connection_to", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionToTool(filepath.Join(root, "definitions", "update_connection_to.json"), svc, log)
		}},
		{"update_connection_expires_at", func() (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionExpiresAtTool(filepath.Join(root, "definitions", "update_connection_expires_at.json"), svc, log)
		}},
	}

	for _, item := range toolsToRegister {
		tool, err := item.make()
		if err != nil {
			fmt.Fprintf(os.Stderr, "new %s: %v\n", item.name, err)
			return
		}
		if !tool.IsIdempotent() {
			fmt.Fprintf(os.Stderr, "%s must be idempotent\n", item.name)
			return
		}
		if err := exec.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", item.name, err)
			return
		}
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_Connection_TargetedUpdates(t *testing.T) {
	t.Run("update_connection_state", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		before := snapshot(conn)

		newState := map[string]any{"open": false, "alarm": "raised"}
		args := mustMarshal(t, map[string]any{"id": conn.ID, "state": newState})
		fmt.Printf("=== update_connection_state args ===\n%s\n", args)

		executeOK(t, "update_connection_state", "inv-upd-state", "call-upd-state", args)
		got := loadConnection(t, conn.ID)
		fmt.Printf("=== after state ===\n%s\n", mustJSON(got))

		assertJSONEqual(t, "state", got.State, repositories.JSON(mustMarshal(t, newState)))
		assertUnchangedExcept(t, before, got, "state")

		executeOK(t, "update_connection_state", "inv-upd-state-retry", "call-upd-state-retry", args)
		got2 := loadConnection(t, conn.ID)
		assertSameConnection(t, got, got2)
	})

	t.Run("update_connection_details", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		before := snapshot(conn)

		newDetails := map[string]any{"material": "iron", "width": 2}
		args := mustMarshal(t, map[string]any{"id": conn.ID, "details": newDetails})
		fmt.Printf("=== update_connection_details args ===\n%s\n", args)

		executeOK(t, "update_connection_details", "inv-upd-details", "call-upd-details", args)
		got := loadConnection(t, conn.ID)
		fmt.Printf("=== after details ===\n%s\n", mustJSON(got))

		assertJSONEqual(t, "details", got.Details, repositories.JSON(mustMarshal(t, newDetails)))
		assertUnchangedExcept(t, before, got, "details")

		executeOK(t, "update_connection_details", "inv-upd-details-retry", "call-upd-details-retry", args)
		assertSameConnection(t, got, loadConnection(t, conn.ID))
	})

	t.Run("update_connection_description", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		before := snapshot(conn)

		const newDesc = "A reinforced iron hatch replaces the wooden door."
		args := mustMarshal(t, map[string]any{"id": conn.ID, "description": newDesc})
		fmt.Printf("=== update_connection_description args ===\n%s\n", args)

		executeOK(t, "update_connection_description", "inv-upd-desc", "call-upd-desc", args)
		got := loadConnection(t, conn.ID)
		fmt.Printf("=== after description ===\n%s\n", mustJSON(got))

		if got.Description != newDesc {
			t.Fatalf("description: want %q, got %q", newDesc, got.Description)
		}
		assertUnchangedExcept(t, before, got, "description")

		executeOK(t, "update_connection_description", "inv-upd-desc-retry", "call-upd-desc-retry", args)
		assertSameConnection(t, got, loadConnection(t, conn.ID))
	})

	t.Run("update_connection_bidirectional", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		if !conn.Bidirectional {
			t.Fatal("seed must start bidirectional=true")
		}
		before := snapshot(conn)

		// true → false
		argsFalse := mustMarshal(t, map[string]any{"id": conn.ID, "bidirectional": false})
		fmt.Printf("=== update_connection_bidirectional → false ===\n%s\n", argsFalse)
		executeOK(t, "update_connection_bidirectional", "inv-upd-bi-false", "call-upd-bi-false", argsFalse)
		gotFalse := loadConnection(t, conn.ID)
		fmt.Printf("=== after bidirectional=false ===\n%s\n", mustJSON(gotFalse))
		if gotFalse.Bidirectional {
			t.Fatal("bidirectional must be false")
		}
		assertUnchangedExcept(t, before, gotFalse, "bidirectional")

		executeOK(t, "update_connection_bidirectional", "inv-upd-bi-false-retry", "call-upd-bi-false-retry", argsFalse)
		assertSameConnection(t, gotFalse, loadConnection(t, conn.ID))

		// false → true
		beforeTrue := snapshot(gotFalse)
		argsTrue := mustMarshal(t, map[string]any{"id": conn.ID, "bidirectional": true})
		fmt.Printf("=== update_connection_bidirectional → true ===\n%s\n", argsTrue)
		executeOK(t, "update_connection_bidirectional", "inv-upd-bi-true", "call-upd-bi-true", argsTrue)
		gotTrue := loadConnection(t, conn.ID)
		fmt.Printf("=== after bidirectional=true ===\n%s\n", mustJSON(gotTrue))
		if !gotTrue.Bidirectional {
			t.Fatal("bidirectional must be true")
		}
		assertUnchangedExcept(t, beforeTrue, gotTrue, "bidirectional")

		executeOK(t, "update_connection_bidirectional", "inv-upd-bi-true-retry", "call-upd-bi-true-retry", argsTrue)
		assertSameConnection(t, gotTrue, loadConnection(t, conn.ID))
	})

	t.Run("update_connection_to", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		before := snapshot(conn)

		args := mustMarshal(t, map[string]any{"id": conn.ID, "to_id": fixture.altTo.ID})
		fmt.Printf("=== update_connection_to args ===\n%s\n", args)

		executeOK(t, "update_connection_to", "inv-upd-to", "call-upd-to", args)
		got := loadConnection(t, conn.ID)
		fmt.Printf("=== after to_id ===\n%s\n", mustJSON(got))

		if got.ToID != fixture.altTo.ID {
			t.Fatalf("to_id: want %d, got %d", fixture.altTo.ID, got.ToID)
		}
		if got.FromID != fixture.from.ID {
			t.Fatalf("from_id must stay %d, got %d", fixture.from.ID, got.FromID)
		}
		assertUnchangedExcept(t, before, got, "to_id")

		executeOK(t, "update_connection_to", "inv-upd-to-retry", "call-upd-to-retry", args)
		assertSameConnection(t, got, loadConnection(t, conn.ID))
	})

	t.Run("update_connection_expires_at", func(t *testing.T) {
		fixture := seedFixture(t)
		conn := fixture.conn
		if conn.ExpiresAt != nil {
			t.Fatal("seed must start with expires_at=null")
		}
		before := snapshot(conn)

		const expires int64 = 1_800_000_000
		argsSet := mustMarshal(t, map[string]any{"id": conn.ID, "expires_at": expires})
		fmt.Printf("=== update_connection_expires_at set ===\n%s\n", argsSet)

		executeOK(t, "update_connection_expires_at", "inv-upd-exp-set", "call-upd-exp-set", argsSet)
		gotSet := loadConnection(t, conn.ID)
		fmt.Printf("=== after expires_at set ===\n%s\n", mustJSON(gotSet))
		if gotSet.ExpiresAt == nil || *gotSet.ExpiresAt != expires {
			t.Fatalf("expires_at: want %d, got %#v", expires, gotSet.ExpiresAt)
		}
		assertUnchangedExcept(t, before, gotSet, "expires_at")

		executeOK(t, "update_connection_expires_at", "inv-upd-exp-set-retry", "call-upd-exp-set-retry", argsSet)
		assertSameConnection(t, gotSet, loadConnection(t, conn.ID))

		beforeClear := snapshot(gotSet)
		argsClear := mustMarshal(t, map[string]any{"id": conn.ID, "expires_at": nil})
		fmt.Printf("=== update_connection_expires_at clear ===\n%s\n", argsClear)

		executeOK(t, "update_connection_expires_at", "inv-upd-exp-clear", "call-upd-exp-clear", argsClear)
		gotClear := loadConnection(t, conn.ID)
		fmt.Printf("=== after expires_at clear ===\n%s\n", mustJSON(gotClear))
		if gotClear.ExpiresAt != nil {
			t.Fatalf("expires_at must be NULL, got %#v", gotClear.ExpiresAt)
		}
		assertUnchangedExcept(t, beforeClear, gotClear, "expires_at")

		executeOK(t, "update_connection_expires_at", "inv-upd-exp-clear-retry", "call-upd-exp-clear-retry", argsClear)
		assertSameConnection(t, gotClear, loadConnection(t, conn.ID))
	})
}

type fixture struct {
	from  *repolocations.Location
	to    *repolocations.Location
	altTo *repolocations.Location
	conn  *repolocations.Connection
}

func seedFixture(t *testing.T) fixture {
	t.Helper()
	resetDB(t)
	ctx := context.Background()

	from := seedLocation(t, "Watchtower")
	to := seedLocation(t, "Courtyard")
	altTo := seedLocation(t, "Armory")

	conn := &repolocations.Connection{
		FromID:        from.ID,
		ToID:          to.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":true,"known_to_players":false}`),
		Details:       repositories.JSON(`{"material":"wood","secret":false}`),
		Description:   "A heavy wooden door between watchtower and courtyard.",
		ExpiresAt:     nil,
	}
	if err := testRepo.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	fmt.Printf("=== Seeded fixture ===\n  from=%d to=%d altTo=%d conn=%d\n%s\n",
		from.ID, to.ID, altTo.ID, conn.ID, mustJSON(conn))
	return fixture{from: from, to: to, altTo: altTo, conn: conn}
}

func seedLocation(t *testing.T, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		Type:        repolocations.LocationType_STRUCTURE,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed location %q: %v", name, err)
	}
	return loc
}

func executeOK(t *testing.T, tool, invokeID, callID string, args json.RawMessage) {
	t.Helper()
	raw, err := ex.Execute(context.Background(), invokeID, tools.Call{
		ID: callID, Name: tool, Role: types.Role_Master, Args: args,
	})
	if err != nil {
		t.Fatalf("Execute %s: %v", tool, err)
	}
	fmt.Printf("=== %s result ===\n%s\n", tool, raw)

	var out struct {
		ID repositories.ID `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s result: %v", tool, err)
	}
	if out.ID == 0 {
		t.Fatalf("%s returned zero id", tool)
	}
	assertNoJournal(t, invokeID, callID)
}

func loadConnection(t *testing.T, id repositories.ID) *repolocations.Connection {
	t.Helper()
	got, err := testRepo.GetConnection(context.Background(), id)
	if err != nil {
		t.Fatalf("GetConnection(%d): %v", id, err)
	}
	return got
}

type connSnap struct {
	FromID        repositories.ID
	ToID          repositories.ID
	Bidirectional bool
	State         repositories.JSON
	Details       repositories.JSON
	Description   string
	ExpiresAt     *int64
}

func snapshot(c *repolocations.Connection) connSnap {
	var expires *int64
	if c.ExpiresAt != nil {
		v := *c.ExpiresAt
		expires = &v
	}
	return connSnap{
		FromID:        c.FromID,
		ToID:          c.ToID,
		Bidirectional: c.Bidirectional,
		State:         append(repositories.JSON(nil), c.State...),
		Details:       append(repositories.JSON(nil), c.Details...),
		Description:   c.Description,
		ExpiresAt:     expires,
	}
}

func assertUnchangedExcept(t *testing.T, before connSnap, after *repolocations.Connection, changed string) {
	t.Helper()
	checks := map[string]func(){
		"from_id": func() {
			if after.FromID != before.FromID {
				t.Fatalf("from_id changed: %d → %d", before.FromID, after.FromID)
			}
		},
		"to_id": func() {
			if after.ToID != before.ToID {
				t.Fatalf("to_id changed: %d → %d", before.ToID, after.ToID)
			}
		},
		"bidirectional": func() {
			if after.Bidirectional != before.Bidirectional {
				t.Fatalf("bidirectional changed: %v → %v", before.Bidirectional, after.Bidirectional)
			}
		},
		"state": func() {
			if !jsonEqual(after.State, before.State) {
				t.Fatalf("state changed: %s → %s", before.State, after.State)
			}
		},
		"details": func() {
			if !jsonEqual(after.Details, before.Details) {
				t.Fatalf("details changed: %s → %s", before.Details, after.Details)
			}
		},
		"description": func() {
			if after.Description != before.Description {
				t.Fatalf("description changed: %q → %q", before.Description, after.Description)
			}
		},
		"expires_at": func() {
			if !expiresEqual(before.ExpiresAt, after.ExpiresAt) {
				t.Fatalf("expires_at changed: %#v → %#v", before.ExpiresAt, after.ExpiresAt)
			}
		},
	}
	for field, check := range checks {
		if field == changed {
			continue
		}
		check()
	}
}

func assertSameConnection(t *testing.T, a, b *repolocations.Connection) {
	t.Helper()
	if a.ID != b.ID || a.FromID != b.FromID || a.ToID != b.ToID || a.Bidirectional != b.Bidirectional ||
		a.Description != b.Description || !expiresEqual(a.ExpiresAt, b.ExpiresAt) ||
		!jsonEqual(a.State, b.State) || !jsonEqual(a.Details, b.Details) {
		t.Fatalf("connection changed after identical retry:\nbefore=%s\nafter=%s", mustJSON(a), mustJSON(b))
	}
}

func assertJSONEqual(t *testing.T, label string, got, want repositories.JSON) {
	t.Helper()
	if !jsonEqual(got, want) {
		t.Fatalf("%s: want %s, got %s", label, want, got)
	}
}

func expiresEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func assertNoJournal(t *testing.T, invokeID, callID string) {
	t.Helper()
	var n int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ? AND id = ?", invokeID, callID).
		Count(&n).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if n != 0 {
		t.Fatalf("idempotent tool must not journal, got %d rows for %s/%s", n, invokeID, callID)
	}
}

func jsonEqual(a, b []byte) bool {
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	la, _ := json.Marshal(left)
	ra, _ := json.Marshal(right)
	return string(la) == string(ra)
}

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func mustJSON(v any) string {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, connections, locations, location_subtypes, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
