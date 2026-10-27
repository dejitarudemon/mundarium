package connection_cross_hierarchy_test

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
	// A01: deterministic Executor create_connection — Postgres only, no LLM.
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

func TestExecutor_CreateConnection_CrossHierarchy(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	// Town → Tavern → Cellar
	town := seedLocation(t, nil, repolocations.LocationType_SETTLEMENT, "Town", "settlement")
	tavern := seedLocation(t, &town.ID, repolocations.LocationType_STRUCTURE, "Tavern", "inn")
	cellar := seedLocation(t, &tavern.ID, repolocations.LocationType_ROOM, "Cellar", "under the tavern")

	// Castle → Dungeon
	castle := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Castle", "keep")
	dungeon := seedLocation(t, &castle.ID, repolocations.LocationType_ROOM, "Dungeon", "beneath the keep")

	beforeParents := map[repositories.ID]*repositories.ID{
		town.ID:    town.ParentID,
		tavern.ID:  tavern.ParentID,
		cellar.ID:  cellar.ParentID,
		castle.ID:  castle.ParentID,
		dungeon.ID: dungeon.ParentID,
	}
	beforeNames := map[repositories.ID]string{
		town.ID:    town.Name,
		tavern.ID:  tavern.Name,
		cellar.ID:  cellar.Name,
		castle.ID:  castle.Name,
		dungeon.ID: dungeon.Name,
	}

	fmt.Printf("=== Seeded locations ===\n")
	for _, loc := range []*repolocations.Location{town, tavern, cellar, castle, dungeon} {
		parent := "null"
		if loc.ParentID != nil {
			parent = fmt.Sprintf("%d", *loc.ParentID)
		}
		fmt.Printf("  id=%d name=%q parent_id=%s type=%s\n", loc.ID, loc.Name, parent, loc.Type.String())
	}

	const (
		wantDescription = "A narrow secret tunnel from the tavern cellar into the castle dungeon."
		invokeID        = "inv-create-connection-cross-hierarchy"
		callID          = "call-create-connection-cross-hierarchy"
	)
	state := map[string]any{"open": false, "known_to_players": false}
	details := map[string]any{"material": "packed earth", "length_paces": 40}

	args, err := json.Marshal(map[string]any{
		"from_id":       cellar.ID,
		"to_id":         dungeon.ID,
		"bidirectional": false,
		"description":   wantDescription,
		"state":         state,
		"details":       details,
		"expires_at":    nil,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	fmt.Printf("=== create_connection args ===\n%s\n", args)

	raw, err := ex.Execute(ctx, invokeID, tools.Call{
		ID:   callID,
		Name: "create_connection",
		Role: types.Role_Master,
		Args: args,
	})
	if err != nil {
		t.Fatalf("Execute create_connection: %v", err)
	}
	fmt.Printf("=== create_connection result ===\n%s\n", raw)

	var created struct {
		ID     repositories.ID `json:"id"`
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	}
	if err := json.Unmarshal(raw, &created); err != nil {
		t.Fatalf("unmarshal create result: %v\n%s", err, raw)
	}
	if created.ID == 0 || created.FromID != cellar.ID || created.ToID != dungeon.ID {
		t.Fatalf("unexpected create payload: %+v (cellar=%d dungeon=%d)", created, cellar.ID, dungeon.ID)
	}

	var connCount int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&connCount).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if connCount != 1 {
		t.Fatalf("want exactly 1 connection, got %d", connCount)
	}

	got, err := testRepo.GetConnection(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	fmt.Printf("=== Final connection ===\n%s\n", mustJSON(t, got))

	if got.FromID != cellar.ID || got.ToID != dungeon.ID {
		t.Fatalf("endpoints: want %d→%d, got %d→%d", cellar.ID, dungeon.ID, got.FromID, got.ToID)
	}
	if got.Bidirectional {
		t.Fatal("want bidirectional=false")
	}
	if got.Description != wantDescription {
		t.Fatalf("description: want %q, got %q", wantDescription, got.Description)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expires_at must be null, got %#v", got.ExpiresAt)
	}
	assertJSONObjectEqual(t, "state", got.State, state)
	assertJSONObjectEqual(t, "details", got.Details, details)

	for id, wantParent := range beforeParents {
		loc, err := testRepo.GetLocation(ctx, id)
		if err != nil {
			t.Fatalf("GetLocation %d: %v", id, err)
		}
		if !sameOptionalID(loc.ParentID, wantParent) {
			t.Fatalf("location %d parent changed: before=%v after=%v", id, wantParent, loc.ParentID)
		}
		if loc.Name != beforeNames[id] {
			t.Fatalf("location %d name changed: before=%q after=%q", id, beforeNames[id], loc.Name)
		}
	}

	var locCount int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locCount).Error; err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if locCount != 5 {
		t.Fatalf("want 5 locations unchanged, got %d", locCount)
	}

	assertJournalCompleted(t, invokeID, callID, "create_connection")
	assertSingleJournalRow(t, invokeID, callID)
}

func seedLocation(
	t *testing.T,
	parentID *repositories.ID,
	locType repolocations.LocationType,
	name, description string,
) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    parentID,
		Type:        locType,
		Name:        name,
		Description: description,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed %q: %v", name, err)
	}
	return loc
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
	var n int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ? AND id = ?", invokeID, callID).
		Count(&n).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 journal row for %s/%s, got %d", invokeID, callID, n)
	}
}

func assertJSONObjectEqual(t *testing.T, label string, raw repositories.JSON, want map[string]any) {
	t.Helper()
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("%s unmarshal: %v (%s)", label, err, raw)
	}
	wantRaw, err := json.Marshal(want)
	if err != nil {
		t.Fatalf("%s marshal want: %v", label, err)
	}
	var wantNorm map[string]any
	if err := json.Unmarshal(wantRaw, &wantNorm); err != nil {
		t.Fatalf("%s normalize want: %v", label, err)
	}
	gotRaw, _ := json.Marshal(got)
	wantCmp, _ := json.Marshal(wantNorm)
	if string(gotRaw) != string(wantCmp) {
		t.Fatalf("%s mismatch:\n got=%s\nwant=%s", label, gotRaw, wantCmp)
	}
}

func sameOptionalID(a, b *repositories.ID) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
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
