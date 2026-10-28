package connection_full_overwrite_test

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
	// A04: deterministic Executor update_connection full overwrite — Postgres only, no LLM.
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

	updateConn, err := toolslocations.NewUpdateConnectionTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "update_connection.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection: %v\n", err)
		return
	}
	if !updateConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "update_connection must be idempotent")
		return
	}
	if err := exec.Register(updateConn, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register update_connection: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_UpdateConnection_FullOverwrite(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	from := seedLocation(t, "Gatehouse")
	oldTo := seedLocation(t, "Old Courtyard")
	newTo := seedLocation(t, "Ruined Chapel")
	fmt.Printf("=== Seeded locations ===\n  from=%d oldTo=%d newTo=%d\n", from.ID, oldTo.ID, newTo.ID)

	expires := int64(1_700_000_000)
	seed := &repolocations.Connection{
		FromID:        from.ID,
		ToID:          oldTo.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":true,"known_to_players":true,"guard":"alive"}`),
		Details:       repositories.JSON(`{"material":"stone","secret":"sigil"}`),
		Description:   "A sealed bidirectional portal under the gatehouse.",
		ExpiresAt:     &expires,
	}
	if err := testRepo.CreateConnection(ctx, seed); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	fmt.Printf("=== Seeded connection ===\n%s\n", mustJSON(seed))

	const (
		invokeID    = "inv-update-connection-full"
		callID      = "call-update-connection-full"
		newDesc     = "The portal has collapsed into a one-way rubble chute."
		wantConnCnt = 1
	)

	args, err := json.Marshal(map[string]any{
		"id":            seed.ID,
		"to_id":         newTo.ID,
		"bidirectional": false,
		"state":         map[string]any{},
		"details":       map[string]any{},
		"description":   newDesc,
		"expires_at":    nil,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	fmt.Printf("=== update_connection args ===\n%s\n", args)

	raw, err := ex.Execute(ctx, invokeID, tools.Call{
		ID: callID, Name: "update_connection", Role: types.Role_Master, Args: args,
	})
	if err != nil {
		t.Fatalf("Execute update_connection: %v", err)
	}
	fmt.Printf("=== update_connection result ===\n%s\n", raw)

	var out struct {
		ID repositories.ID `json:"id"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if out.ID != seed.ID {
		t.Fatalf("want id=%d, got %d", seed.ID, out.ID)
	}

	got, err := testRepo.GetConnection(ctx, seed.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	fmt.Printf("=== Connection after full overwrite ===\n%s\n", mustJSON(got))

	if got.FromID != from.ID {
		t.Fatalf("from_id must stay %d, got %d", from.ID, got.FromID)
	}
	if got.ToID != newTo.ID {
		t.Fatalf("to_id: want %d, got %d", newTo.ID, got.ToID)
	}
	if got.Bidirectional {
		t.Fatal("bidirectional must be false after overwrite (GORM must persist false)")
	}
	if got.Description != newDesc {
		t.Fatalf("description: want %q, got %q", newDesc, got.Description)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expires_at must be NULL, got %#v", got.ExpiresAt)
	}

	assertEmptyJSONObject(t, "state", got.State)
	assertEmptyJSONObject(t, "details", got.Details)
	assertNoKey(t, "state", got.State, "open", "known_to_players", "guard")
	assertNoKey(t, "details", got.Details, "material", "secret")

	var connCount int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&connCount).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if connCount != wantConnCnt {
		t.Fatalf("want %d connections, got %d", wantConnCnt, connCount)
	}

	assertNoJournal(t, invokeID, callID)

	// Idempotent retry with the same full payload must keep the same final state.
	raw2, err := ex.Execute(ctx, invokeID+"-retry", tools.Call{
		ID: callID + "-retry", Name: "update_connection", Role: types.Role_Master, Args: args,
	})
	if err != nil {
		t.Fatalf("retry update_connection: %v", err)
	}
	fmt.Printf("=== update_connection retry result ===\n%s\n", raw2)

	got2, err := testRepo.GetConnection(ctx, seed.ID)
	if err != nil {
		t.Fatalf("GetConnection after retry: %v", err)
	}
	if got2.FromID != from.ID || got2.ToID != newTo.ID || got2.Bidirectional || got2.ExpiresAt != nil || got2.Description != newDesc {
		t.Fatalf("retry must keep overwritten state, got %+v", got2)
	}
	assertEmptyJSONObject(t, "state-retry", got2.State)
	assertEmptyJSONObject(t, "details-retry", got2.Details)
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

func assertEmptyJSONObject(t *testing.T, label string, raw repositories.JSON) {
	t.Helper()
	if len(raw) == 0 || string(raw) == "null" {
		t.Fatalf("%s: want empty JSON object {}, got %q", label, raw)
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s unmarshal: %v (%s)", label, err, raw)
	}
	if len(obj) != 0 {
		t.Fatalf("%s: want empty object, got %s", label, raw)
	}
}

func assertNoKey(t *testing.T, label string, raw repositories.JSON, keys ...string) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s unmarshal: %v", label, err)
	}
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			t.Fatalf("%s still contains old key %q: %s", label, key, raw)
		}
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
		t.Fatalf("update_connection is idempotent and must not journal, got %d rows", n)
	}
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
