package connection_call_replay_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// A07: deterministic Executor create_connection call replay — Postgres only, no LLM.
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

	createConn, err := toolslocations.NewCreateConnectionTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "create_connection.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_connection: %v\n", err)
		return
	}
	if createConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "create_connection must be non-idempotent")
		return
	}
	if err := exec.Register(createConn, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register create_connection: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_CreateConnection_CallReplay(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	from := seedLocation(t, "Cellar")
	to := seedLocation(t, "Dungeon")
	fmt.Printf("=== Seeded locations ===\n  from=%d to=%d\n", from.ID, to.ID)

	const (
		invokeID = "inv-create-connection-replay"
		callID   = "call-create-connection-replay"
	)

	args := mustMarshal(t, map[string]any{
		"from_id":       from.ID,
		"to_id":         to.ID,
		"bidirectional": false,
		"description":   "A secret tunnel between cellar and dungeon.",
		"state":         map[string]any{"open": false},
		"details":       map[string]any{"material": "earth"},
		"expires_at":    nil,
	})
	call := tools.Call{ID: callID, Name: "create_connection", Role: types.Role_Master, Args: args}
	fmt.Printf("=== create_connection args ===\n%s\n", args)

	first, err := ex.Execute(ctx, invokeID, call)
	if err != nil {
		t.Fatalf("first create_connection: %v", err)
	}
	fmt.Printf("=== first result ===\n%s\n", first)
	assertJournalCompleted(t, invokeID, callID, "create_connection")

	var created struct {
		ID     repositories.ID `json:"id"`
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	}
	if err := json.Unmarshal(first, &created); err != nil {
		t.Fatalf("unmarshal first: %v", err)
	}
	if created.ID == 0 || created.FromID != from.ID || created.ToID != to.ID {
		t.Fatalf("unexpected first payload: %+v", created)
	}

	connCountBefore := countConnections(t)
	if connCountBefore != 1 {
		t.Fatalf("want 1 connection after first create, got %d", connCountBefore)
	}
	journalCountBefore := countJournal(t, invokeID, callID)
	if journalCountBefore != 1 {
		t.Fatalf("want 1 journal row after first create, got %d", journalCountBefore)
	}

	second, err := ex.Execute(ctx, invokeID, call)
	if err != nil {
		t.Fatalf("replay create_connection: %v", err)
	}
	fmt.Printf("=== replay result ===\n%s\n", second)
	assertJSONEqual(t, first, second)

	connCountAfter := countConnections(t)
	if connCountAfter != connCountBefore {
		t.Fatalf("replay must not create another connection: before=%d after=%d", connCountBefore, connCountAfter)
	}
	assertJournalCompleted(t, invokeID, callID, "create_connection")
	assertSingleJournalRow(t, invokeID, callID)

	got, err := testRepo.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	fmt.Printf("=== Connection after replay ===\n%s\n", mustJSON(got))
	if got.FromID != from.ID || got.ToID != to.ID || got.Description != "A secret tunnel between cellar and dungeon." {
		t.Fatalf("connection corrupted after replay: %+v", got)
	}

	// Same args with a different call_id must create a second row — no arg-based dedup.
	t.Run("different_call_id_same_args_creates_again", func(t *testing.T) {
		const (
			invokeID2 = "inv-create-connection-other-call"
			callID2   = "call-create-connection-other-call"
		)
		otherCall := tools.Call{ID: callID2, Name: "create_connection", Role: types.Role_Master, Args: args}
		fmt.Printf("=== create_connection different call_id same args ===\ninvoke=%s call=%s\n", invokeID2, callID2)

		raw, err := ex.Execute(ctx, invokeID2, otherCall)
		if err != nil {
			t.Fatalf("create_connection with new call_id: %v", err)
		}
		fmt.Printf("=== different call_id result ===\n%s\n", raw)

		var secondCreated struct {
			ID repositories.ID `json:"id"`
		}
		if err := json.Unmarshal(raw, &secondCreated); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if secondCreated.ID == 0 || secondCreated.ID == created.ID {
			t.Fatalf("want a new connection id distinct from %d, got %d", created.ID, secondCreated.ID)
		}

		if n := countConnections(t); n != 2 {
			t.Fatalf("want 2 connections after distinct call_id, got %d", n)
		}
		assertJournalCompleted(t, invokeID2, callID2, "create_connection")
		assertSingleJournalRow(t, invokeID2, callID2)
		// Original journal identity untouched.
		assertSingleJournalRow(t, invokeID, callID)
	})
}

func seedLocation(t *testing.T, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		Type:        repolocations.LocationType_ROOM,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed location %q: %v", name, err)
	}
	return loc
}

func countConnections(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&n).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	return n
}

func countJournal(t *testing.T, invokeID, callID string) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ? AND id = ?", invokeID, callID).
		Count(&n).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	return n
}

func assertJournalCompleted(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal %s/%s: %v", invokeID, callID, err)
	}
	fmt.Printf("=== Journal ===\nid=%s group=%s tool=%s status=%s\n", req.ID, req.GroupID, req.Tool, req.Status)
	if req.Tool != tool {
		t.Fatalf("journal tool: want %s, got %s", tool, req.Tool)
	}
	if req.Status != journal.Status_Completed {
		t.Fatalf("journal status: want %s, got %s err=%v", journal.Status_Completed, req.Status, req.Error)
	}
	if req.Result.Data == nil {
		t.Fatal("completed journal must store result data")
	}
}

func assertSingleJournalRow(t *testing.T, invokeID, callID string) {
	t.Helper()
	n := countJournal(t, invokeID, callID)
	if n != 1 {
		t.Fatalf("want exactly 1 journal row for %s/%s, got %d", invokeID, callID, n)
	}
}

func assertJSONEqual(t *testing.T, a, b json.RawMessage) {
	t.Helper()
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		t.Fatalf("unmarshal first: %v\n%s", err, a)
	}
	if err := json.Unmarshal(b, &right); err != nil {
		t.Fatalf("unmarshal second: %v\n%s", err, b)
	}
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	if string(leftRaw) != string(rightRaw) {
		t.Fatalf("replay payload mismatch:\nfirst=%s\nsecond=%s", a, b)
	}
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
