package connection_survives_location_move_test

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
	// A08: Connection survives location move — Postgres only, no LLM.
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

	moveLoc, err := toolslocations.NewMoveLocationTool(
		filepath.Join(root, "definitions", "move_location.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new move_location: %v\n", err)
		return
	}
	if !moveLoc.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "move_location must be idempotent")
		return
	}

	for _, tool := range []tools.Tool{createConn, findConn, moveLoc} {
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

func TestExecutor_Connection_SurvivesLocationMove(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Town → Tavern → Cellar
	town := seedLocation(t, nil, repolocations.LocationType_SETTLEMENT, "Town")
	tavern := seedLocation(t, &town.ID, repolocations.LocationType_STRUCTURE, "Tavern")
	cellar := seedLocation(t, &tavern.ID, repolocations.LocationType_ROOM, "Cellar")

	// Castle → Dungeon  (separate branch)
	castle := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Castle")
	dungeon := seedLocation(t, &castle.ID, repolocations.LocationType_ROOM, "Dungeon")

	fmt.Printf("=== Seeded hierarchy ===\n")
	for _, loc := range []*repolocations.Location{town, tavern, cellar, castle, dungeon} {
		fmt.Printf("  id=%d name=%q parent=%v\n", loc.ID, loc.Name, loc.ParentID)
	}

	const (
		wantDesc = "A secret tunnel from the tavern cellar into the castle dungeon."
		createInv = "inv-create-tunnel"
		createCall = "call-create-tunnel"
	)
	state := map[string]any{"open": false, "known_to_players": false}
	details := map[string]any{"material": "packed earth", "length_paces": 40}

	createArgs := mustMarshal(t, map[string]any{
		"from_id":       cellar.ID,
		"to_id":         dungeon.ID,
		"bidirectional": false,
		"description":   wantDesc,
		"state":         state,
		"details":       details,
		"expires_at":    nil,
	})
	fmt.Printf("=== create_connection args ===\n%s\n", createArgs)

	createRaw, err := ex.Execute(ctx, createInv, tools.Call{
		ID: createCall, Name: "create_connection", Role: types.Role_Master, Args: createArgs,
	})
	if err != nil {
		t.Fatalf("create_connection: %v", err)
	}
	fmt.Printf("=== create_connection result ===\n%s\n", createRaw)

	var created struct {
		ID     repositories.ID `json:"id"`
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	}
	if err := json.Unmarshal(createRaw, &created); err != nil {
		t.Fatalf("unmarshal create: %v", err)
	}
	if created.ID == 0 || created.FromID != cellar.ID || created.ToID != dungeon.ID {
		t.Fatalf("unexpected create payload: %+v", created)
	}
	assertJournalCompleted(t, createInv, createCall, "create_connection")

	beforeConn, err := testRepo.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection before move: %v", err)
	}
	fmt.Printf("=== Connection before move ===\n%s\n", mustJSON(beforeConn))

	// Reparent Dungeon under Tavern — hierarchy changes, Connection must stay on same IDs.
	moveArgs := mustMarshal(t, map[string]any{
		"id":        dungeon.ID,
		"parent_id": tavern.ID,
	})
	fmt.Printf("=== move_location args ===\n%s\n", moveArgs)

	moveRaw, err := ex.Execute(ctx, "inv-move-dungeon", tools.Call{
		ID: "call-move-dungeon", Name: "move_location", Role: types.Role_Master, Args: moveArgs,
	})
	if err != nil {
		t.Fatalf("move_location: %v", err)
	}
	fmt.Printf("=== move_location result ===\n%s\n", moveRaw)
	assertNoJournal(t, "inv-move-dungeon", "call-move-dungeon")

	moved, err := testRepo.GetLocation(ctx, dungeon.ID)
	if err != nil {
		t.Fatalf("GetLocation dungeon after move: %v", err)
	}
	fmt.Printf("=== Dungeon after move ===\nid=%d parent_id=%v name=%q\n", moved.ID, moved.ParentID, moved.Name)
	if moved.ParentID == nil || *moved.ParentID != tavern.ID {
		t.Fatalf("dungeon parent: want %d, got %v", tavern.ID, moved.ParentID)
	}
	if moved.Name != "Dungeon" {
		t.Fatalf("dungeon name must stay, got %q", moved.Name)
	}

	// Other parents unchanged.
	assertParent(t, town.ID, nil)
	assertParent(t, tavern.ID, &town.ID)
	assertParent(t, cellar.ID, &tavern.ID)
	assertParent(t, castle.ID, nil)

	afterConn, err := testRepo.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection after move: %v", err)
	}
	fmt.Printf("=== Connection after move ===\n%s\n", mustJSON(afterConn))

	if afterConn.ID != beforeConn.ID || afterConn.FromID != cellar.ID || afterConn.ToID != dungeon.ID {
		t.Fatalf("connection endpoints must stay %d→%d id=%d, got id=%d %d→%d",
			cellar.ID, dungeon.ID, beforeConn.ID, afterConn.ID, afterConn.FromID, afterConn.ToID)
	}
	if afterConn.Bidirectional != beforeConn.Bidirectional {
		t.Fatalf("bidirectional changed: %v → %v", beforeConn.Bidirectional, afterConn.Bidirectional)
	}
	if afterConn.Description != wantDesc {
		t.Fatalf("description changed: %q", afterConn.Description)
	}
	if !jsonEqual(afterConn.State, beforeConn.State) {
		t.Fatalf("state changed:\nbefore=%s\nafter=%s", beforeConn.State, afterConn.State)
	}
	if !jsonEqual(afterConn.Details, beforeConn.Details) {
		t.Fatalf("details changed:\nbefore=%s\nafter=%s", beforeConn.Details, afterConn.Details)
	}
	if !expiresEqual(afterConn.ExpiresAt, beforeConn.ExpiresAt) {
		t.Fatalf("expires_at changed: %#v → %#v", beforeConn.ExpiresAt, afterConn.ExpiresAt)
	}

	var connCount int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&connCount).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if connCount != 1 {
		t.Fatalf("want exactly 1 connection, got %d", connCount)
	}

	findArgs := mustMarshal(t, map[string]any{
		"from_id": cellar.ID,
		"to_id":   dungeon.ID,
	})
	fmt.Printf("=== find_connections args ===\n%s\n", findArgs)
	findRaw, err := ex.Execute(ctx, "inv-find-after-move", tools.Call{
		ID: "call-find-after-move", Name: "find_connections", Role: types.Role_Master, Args: findArgs,
	})
	if err != nil {
		t.Fatalf("find_connections: %v", err)
	}
	fmt.Printf("=== find_connections result ===\n%s\n", findRaw)

	var found struct {
		Connections []struct {
			ID            repositories.ID   `json:"id"`
			FromID        repositories.ID   `json:"from_id"`
			ToID          repositories.ID   `json:"to_id"`
			Bidirectional bool              `json:"bidirectional"`
			Description   string            `json:"description"`
			State         repositories.JSON `json:"state"`
			Details       repositories.JSON `json:"details"`
		} `json:"connections"`
	}
	if err := json.Unmarshal(findRaw, &found); err != nil {
		t.Fatalf("unmarshal find: %v", err)
	}
	if len(found.Connections) != 1 {
		t.Fatalf("want 1 connection via find, got %#v", found.Connections)
	}
	c := found.Connections[0]
	if c.ID != created.ID || c.FromID != cellar.ID || c.ToID != dungeon.ID || c.Description != wantDesc {
		t.Fatalf("find payload mismatch: %+v", c)
	}
	if c.Bidirectional || !jsonEqual(c.State, beforeConn.State) || !jsonEqual(c.Details, beforeConn.Details) {
		t.Fatalf("find fields drifted: %+v", c)
	}
	assertNoJournal(t, "inv-find-after-move", "call-find-after-move")
}

func seedLocation(t *testing.T, parentID *repositories.ID, locType repolocations.LocationType, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    parentID,
		Type:        locType,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed %q: %v", name, err)
	}
	return loc
}

func assertParent(t *testing.T, id repositories.ID, want *repositories.ID) {
	t.Helper()
	loc, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation %d: %v", id, err)
	}
	if want == nil {
		if loc.ParentID != nil {
			t.Fatalf("location %d: want root, got parent=%d", id, *loc.ParentID)
		}
		return
	}
	if loc.ParentID == nil || *loc.ParentID != *want {
		t.Fatalf("location %d: want parent=%d, got %v", id, *want, loc.ParentID)
	}
}

func assertJournalCompleted(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	fmt.Printf("=== Journal ===\nid=%s group=%s tool=%s status=%s\n", req.ID, req.GroupID, req.Tool, req.Status)
	if req.Tool != tool || req.Status != journal.Status_Completed {
		t.Fatalf("journal: tool=%s status=%s err=%v", req.Tool, req.Status, req.Error)
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

func expiresEqual(a, b *int64) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
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
