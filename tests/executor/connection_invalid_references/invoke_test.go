package connection_invalid_references_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
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

const missingID repositories.ID = 999999

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// A06: deterministic Executor connection invalid references — Postgres only, no LLM.
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

	createConn, err := toolslocations.NewCreateConnectionTool(
		filepath.Join(root, "definitions", "create_connection.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_connection: %v\n", err)
		return
	}
	if createConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "create_connection must be non-idempotent")
		return
	}

	findConn, err := toolslocations.NewFindConnectionsTool(
		filepath.Join(root, "definitions", "find_connections.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_connections: %v\n", err)
		return
	}
	if !findConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "find_connections must be idempotent")
		return
	}

	updateTo, err := toolslocations.NewUpdateConnectionToTool(
		filepath.Join(root, "definitions", "update_connection_to.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection_to: %v\n", err)
		return
	}
	if !updateTo.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "update_connection_to must be idempotent")
		return
	}

	updateConn, err := toolslocations.NewUpdateConnectionTool(
		filepath.Join(root, "definitions", "update_connection.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection: %v\n", err)
		return
	}
	if !updateConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "update_connection must be idempotent")
		return
	}

	for _, tool := range []tools.Tool{createConn, findConn, updateTo, updateConn} {
		if err := exec.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", tool.Definition().Name, err)
			return
		}
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_Connection_InvalidReferences(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	from := seedLocation(t, "Gate")
	to := seedLocation(t, "Yard")
	conn := &repolocations.Connection{
		FromID:        from.ID,
		ToID:          to.ID,
		Bidirectional: false,
		State:         repositories.JSON(`{"open":true}`),
		Details:       repositories.JSON(`{"material":"wood"}`),
		Description:   "seed gate→yard",
	}
	if err := testRepo.CreateConnection(ctx, conn); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	fmt.Printf("=== Seeded ===\n  from=%d to=%d conn=%d\n", from.ID, to.ID, conn.ID)
	before := snapshotState(t)

	t.Run("create_missing_from_id", func(t *testing.T) {
		args := mustMarshal(t, map[string]any{
			"from_id":       missingID,
			"to_id":         to.ID,
			"bidirectional": false,
			"description":   "orphan from",
			"state":         map[string]any{},
			"details":       map[string]any{},
			"expires_at":    nil,
		})
		fmt.Printf("=== create_connection missing from_id ===\n%s\n", args)
		mustFailCreate(t, "inv-create-bad-from", "call-create-bad-from", args, before)
	})

	t.Run("create_missing_to_id", func(t *testing.T) {
		args := mustMarshal(t, map[string]any{
			"from_id":       from.ID,
			"to_id":         missingID,
			"bidirectional": false,
			"description":   "orphan to",
			"state":         map[string]any{},
			"details":       map[string]any{},
			"expires_at":    nil,
		})
		fmt.Printf("=== create_connection missing to_id ===\n%s\n", args)
		mustFailCreate(t, "inv-create-bad-to", "call-create-bad-to", args, before)
	})

	t.Run("update_to_missing_location", func(t *testing.T) {
		args := mustMarshal(t, map[string]any{
			"id":    conn.ID,
			"to_id": missingID,
		})
		fmt.Printf("=== update_connection_to missing to_id ===\n%s\n", args)
		mustFailIdempotent(t, "update_connection_to", "inv-upd-to-missing", "call-upd-to-missing", args, before, wantReferenceError)
	})

	// Full overwrite path (PutConnection) must reject missing to_id the same way as PatchConnectionTo.
	t.Run("update_full_with_missing_to_id", func(t *testing.T) {
		args := mustMarshal(t, map[string]any{
			"id":            conn.ID,
			"to_id":         missingID,
			"bidirectional": true,
			"state":         map[string]any{"open": false, "corrupted": true},
			"details":       map[string]any{"material": "invalid"},
			"description":   "This update must not persist",
			"expires_at":    nil,
		})
		fmt.Printf("=== update_connection missing to_id (PutConnection) ===\n%s\n", args)
		mustFailIdempotent(t, "update_connection", "inv-upd-full-bad-to", "call-upd-full-bad-to", args, before, wantReferenceError)

		got, err := testRepo.GetConnection(ctx, conn.ID)
		if err != nil {
			t.Fatalf("GetConnection after failed full update: %v", err)
		}
		fmt.Printf("=== Connection after failed update_connection ===\n%s\n", mustJSON(got))
		if got.FromID != from.ID || got.ToID != to.ID {
			t.Fatalf("endpoints must stay %d→%d, got %d→%d", from.ID, to.ID, got.FromID, got.ToID)
		}
		if got.Bidirectional {
			t.Fatal("bidirectional must stay false")
		}
		if got.Description != "seed gate→yard" {
			t.Fatalf("description must stay seed value, got %q", got.Description)
		}
		if !jsonEqual(got.State, []byte(`{"open":true}`)) {
			t.Fatalf("state must stay seed value, got %s", got.State)
		}
		if !jsonEqual(got.Details, []byte(`{"material":"wood"}`)) {
			t.Fatalf("details must stay seed value, got %s", got.Details)
		}
	})

	t.Run("update_missing_connection", func(t *testing.T) {
		args := mustMarshal(t, map[string]any{
			"id":            missingID,
			"to_id":         to.ID,
			"bidirectional": false,
			"state":         map[string]any{},
			"details":       map[string]any{},
			"description":   "ghost connection",
			"expires_at":    nil,
		})
		fmt.Printf("=== update_connection missing id ===\n%s\n", args)
		mustFailIdempotent(t, "update_connection", "inv-upd-missing", "call-upd-missing", args, before, wantNotFoundError)
	})

	t.Run("find_missing_ids_empty", func(t *testing.T) {
		// Service contract: unknown filter ids are a valid query and return [].
		cases := []struct {
			name string
			args map[string]any
		}{
			{"from_id", map[string]any{"from_id": missingID}},
			{"to_id", map[string]any{"to_id": missingID}},
			{"location_id", map[string]any{"location_id": missingID}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				args := mustMarshal(t, tc.args)
				fmt.Printf("=== find_connections %s ===\n%s\n", tc.name, args)
				raw, err := ex.Execute(ctx, "inv-find-missing-"+tc.name, tools.Call{
					ID: "call-find-missing-" + tc.name, Name: "find_connections", Role: types.Role_Master, Args: args,
				})
				if err != nil {
					t.Fatalf("find_connections with missing %s must succeed with empty list, got err=%v", tc.name, err)
				}
				fmt.Printf("=== find_connections result ===\n%s\n", raw)
				var out struct {
					Connections []json.RawMessage `json:"connections"`
				}
				if err := json.Unmarshal(raw, &out); err != nil {
					t.Fatalf("unmarshal: %v", err)
				}
				if len(out.Connections) != 0 {
					t.Fatalf("want empty connections, got %s", raw)
				}
				assertNoJournal(t, "inv-find-missing-"+tc.name, "call-find-missing-"+tc.name)
				assertStateUnchanged(t, before)
			})
		}
	})

	// Seed connection must still be intact after all mutation failures.
	got, err := testRepo.GetConnection(ctx, conn.ID)
	if err != nil {
		t.Fatalf("seed connection after cases: %v", err)
	}
	if got.FromID != from.ID || got.ToID != to.ID || got.Description != "seed gate→yard" {
		t.Fatalf("seed connection corrupted: %+v", got)
	}
}

func mustFailCreate(t *testing.T, invokeID, callID string, args json.RawMessage, before stateSnap) {
	t.Helper()
	raw, err := ex.Execute(context.Background(), invokeID, tools.Call{
		ID: callID, Name: "create_connection", Role: types.Role_Master, Args: args,
	})
	if err == nil {
		t.Fatalf("create_connection must fail, got raw=%s", raw)
	}
	if len(raw) != 0 {
		t.Fatalf("failed create must not return payload, got %s", raw)
	}
	fmt.Printf("=== create_connection error ===\n%v\n", err)
	wantReferenceError(t, err)
	assertStateUnchanged(t, before)
	assertJournalFailed(t, invokeID, callID, "create_connection")
}

func mustFailIdempotent(
	t *testing.T,
	tool, invokeID, callID string,
	args json.RawMessage,
	before stateSnap,
	checkErr func(*testing.T, error),
) {
	t.Helper()
	raw, err := ex.Execute(context.Background(), invokeID, tools.Call{
		ID: callID, Name: tool, Role: types.Role_Master, Args: args,
	})
	if err == nil {
		t.Fatalf("%s must fail, got raw=%s", tool, raw)
	}
	if len(raw) != 0 {
		t.Fatalf("%s failed call must not return payload, got %s", tool, raw)
	}
	fmt.Printf("=== %s error ===\n%v\n", tool, err)
	checkErr(t, err)
	assertStateUnchanged(t, before)
	assertNoJournal(t, invokeID, callID)
}

func wantReferenceError(t *testing.T, err error) {
	t.Helper()
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "invalid request") &&
		!strings.Contains(errText, "foreign") &&
		!strings.Contains(errText, "violat") &&
		!strings.Contains(errText, "constraint") &&
		!strings.Contains(errText, "23503") {
		t.Fatalf("want FK/reference InvalidRequest, got: %v", err)
	}
}

func wantNotFoundError(t *testing.T, err error) {
	t.Helper()
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "not found") && !strings.Contains(errText, "record") {
		t.Fatalf("want not-found style error, got: %v", err)
	}
}

func assertJournalFailed(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	fmt.Printf("=== Journal ===\nid=%s group=%s tool=%s status=%s\n", req.ID, req.GroupID, req.Tool, req.Status)
	if req.Tool != tool {
		t.Fatalf("journal tool: want %s, got %s", tool, req.Tool)
	}
	if req.Status != journal.Status_Failed {
		t.Fatalf("journal status: want %s, got %s (error=%v)", journal.Status_Failed, req.Status, req.Error)
	}
	if req.Error.Err() == nil {
		t.Fatal("failed journal row must store error")
	}
	if req.Status == journal.Status_Completed {
		t.Fatal("integrity failure must not be classified as successful")
	}
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

type stateSnap struct {
	Locations   int64
	Connections int64
	Dump        string
}

func snapshotState(t *testing.T) stateSnap {
	t.Helper()
	var locs, conns int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locs).Error; err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&conns).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	var all []repolocations.Connection
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("dump connections: %v", err)
	}
	raw, err := json.Marshal(all)
	if err != nil {
		t.Fatalf("marshal dump: %v", err)
	}
	return stateSnap{Locations: locs, Connections: conns, Dump: string(raw)}
}

func assertStateUnchanged(t *testing.T, before stateSnap) {
	t.Helper()
	after := snapshotState(t)
	if after.Locations != before.Locations || after.Connections != before.Connections || after.Dump != before.Dump {
		t.Fatalf("DB mutated:\nbefore locs=%d conns=%d dump=%s\nafter  locs=%d conns=%d dump=%s",
			before.Locations, before.Connections, before.Dump,
			after.Locations, after.Connections, after.Dump)
	}
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

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, connections, locations, location_subtypes, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
