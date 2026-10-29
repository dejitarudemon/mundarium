package connection_direction_filters_test

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
	// A02: deterministic Executor find_connections filters — Postgres only, no LLM.
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

	findConn, err := toolslocations.NewFindConnectionsTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "find_connections.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_connections: %v\n", err)
		return
	}
	if !findConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "find_connections must be idempotent")
		return
	}
	if err := exec.Register(findConn, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register find_connections: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_FindConnections_DirectionFilters(t *testing.T) {
	resetDB(t)

	a := seedLocation(t, "A")
	b := seedLocation(t, "B")
	c := seedLocation(t, "C")

	// One-way A→B, bidirectional B→A, one-way A→C, one-way C→B (no reverse C↔B or C↔A).
	// C→B (bidirectional=false) is the fixture that separates either_direction from
	// include_bidirectional_reverse: searching B→C finds C→B only with either_direction.
	ab := seedConnection(t, a.ID, b.ID, "ab-one-way", false)
	ba := seedConnection(t, b.ID, a.ID, "ba-bidirectional", true)
	ac := seedConnection(t, a.ID, c.ID, "ac-one-way", false)
	cb := seedConnection(t, c.ID, b.ID, "cb-one-way", false)

	seedCount := connectionCount(t)

	fmt.Printf("=== Seeded locations ===\n  A=%d B=%d C=%d\n", a.ID, b.ID, c.ID)
	fmt.Printf("=== Seeded connections (count=%d) ===\n", seedCount)
	for _, conn := range []*repolocations.Connection{ab, ba, ac, cb} {
		fmt.Printf("  id=%d %d→%d bidirectional=%v desc=%q\n",
			conn.ID, conn.FromID, conn.ToID, conn.Bidirectional, conn.Description)
	}

	t.Run("directed_A_to_B", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"from_id": a.ID,
			"to_id":   b.ID,
		})
		assertExactIDs(t, ids, ab.ID)
	})

	t.Run("directed_B_to_A", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"from_id": b.ID,
			"to_id":   a.ID,
		})
		assertExactIDs(t, ids, ba.ID)
	})

	t.Run("include_bidirectional_reverse_A_B", func(t *testing.T) {
		// Looking A→B also picks B→A because that reverse row is bidirectional.
		ids := findIDs(t, seedCount, map[string]any{
			"from_id":                       a.ID,
			"to_id":                         b.ID,
			"include_bidirectional_reverse": true,
		})
		assertExactIDs(t, ids, ab.ID, ba.ID)
	})

	t.Run("either_direction_A_B", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"from_id":          a.ID,
			"to_id":            b.ID,
			"either_direction": true,
		})
		assertExactIDs(t, ids, ab.ID, ba.ID)
	})

	// Distinguish either_direction vs include_bidirectional_reverse using one-way C→B.
	t.Run("either_direction_finds_one_way_reverse_C_B", func(t *testing.T) {
		// Searching B→C with either_direction must still surface the stored C→B row.
		// This is a search orientation mode, not a claim that B can travel to C.
		filter := map[string]any{
			"from_id":          b.ID,
			"to_id":            c.ID,
			"either_direction": true,
		}
		ids := findIDs(t, seedCount, filter)
		fmt.Printf("=== either_direction B→C: args=%s got=%v want=[%d]\n", mustJSON(filter), ids, cb.ID)
		assertExactIDs(t, ids, cb.ID)
	})

	t.Run("bidirectional_reverse_ignores_one_way_reverse_C_B", func(t *testing.T) {
		// Same B→C query with include_bidirectional_reverse must stay empty:
		// reverse C→B exists but is not bidirectional.
		filter := map[string]any{
			"from_id":                       b.ID,
			"to_id":                         c.ID,
			"include_bidirectional_reverse": true,
		}
		ids := findIDs(t, seedCount, filter)
		fmt.Printf("=== include_bidirectional_reverse B→C: args=%s got=%v want=[]\n", mustJSON(filter), ids)
		assertExactIDs(t, ids)
	})

	t.Run("bidirectional_reverse_keeps_direct_one_way_C_B", func(t *testing.T) {
		filter := map[string]any{
			"from_id":                       c.ID,
			"to_id":                         b.ID,
			"include_bidirectional_reverse": true,
		}
		ids := findIDs(t, seedCount, filter)
		fmt.Printf("=== include_bidirectional_reverse C→B: args=%s got=%v want=[%d]\n", mustJSON(filter), ids, cb.ID)
		assertExactIDs(t, ids, cb.ID)
	})

	t.Run("location_id_A", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"location_id": a.ID,
		})
		assertExactIDs(t, ids, ab.ID, ba.ID, ac.ID)
	})

	t.Run("missing_pair_empty", func(t *testing.T) {
		// Empty Connection search ≠ unreachable in the world; only no recorded C→A link.
		ids := findIDs(t, seedCount, map[string]any{
			"from_id": c.ID,
			"to_id":   a.ID,
		})
		assertExactIDs(t, ids)
	})

	t.Run("location_id_overrides_direction", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"from_id":                       c.ID,
			"to_id":                         a.ID,
			"location_id":                   a.ID,
			"either_direction":              true,
			"include_bidirectional_reverse": true,
		})
		assertExactIDs(t, ids, ab.ID, ba.ID, ac.ID)
	})

	t.Run("no_duplicates", func(t *testing.T) {
		ids := findIDs(t, seedCount, map[string]any{
			"from_id":                       a.ID,
			"to_id":                         b.ID,
			"either_direction":              true,
			"include_bidirectional_reverse": true,
		})
		assertExactIDs(t, ids, ab.ID, ba.ID)
	})

	if got := connectionCount(t); got != seedCount {
		t.Fatalf("find_connections must not mutate Connection rows: before=%d after=%d", seedCount, got)
	}
}

type findResult struct {
	Connections []struct {
		ID     repositories.ID `json:"id"`
		FromID repositories.ID `json:"from_id"`
		ToID   repositories.ID `json:"to_id"`
	} `json:"connections"`
}

func findIDs(t *testing.T, seedCount int64, filter map[string]any) []repositories.ID {
	t.Helper()

	args, err := json.Marshal(filter)
	if err != nil {
		t.Fatalf("marshal filter: %v", err)
	}
	fmt.Printf("--- find_connections %s args=%s\n", t.Name(), args)

	invokeID := "inv-find-" + t.Name()
	callID := "call-find-" + t.Name()
	raw, err := ex.Execute(context.Background(), invokeID, tools.Call{
		ID:   callID,
		Name: "find_connections",
		Role: types.Role_Master,
		Args: args,
	})
	if err != nil {
		t.Fatalf("Execute find_connections: %v", err)
	}
	fmt.Printf("--- find_connections %s result=%s\n", t.Name(), raw)

	var out findResult
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, raw)
	}

	ids := make([]repositories.ID, 0, len(out.Connections))
	seen := map[repositories.ID]struct{}{}
	for _, c := range out.Connections {
		if _, ok := seen[c.ID]; ok {
			t.Fatalf("duplicate connection id %d in result %#v", c.ID, out.Connections)
		}
		seen[c.ID] = struct{}{}
		ids = append(ids, c.ID)
	}

	assertNoJournal(t, invokeID, callID)
	if got := connectionCount(t); got != seedCount {
		t.Fatalf("find must not create/delete connections: before=%d after=%d", seedCount, got)
	}
	return ids
}

func assertExactIDs(t *testing.T, got []repositories.ID, want ...repositories.ID) {
	t.Helper()
	fmt.Printf("--- assertExactIDs %s got=%v want=%v\n", t.Name(), got, want)
	if len(got) != len(want) {
		t.Fatalf("want %d ids %v, got %d %v", len(want), want, len(got), got)
	}
	wantSet := map[repositories.ID]struct{}{}
	for _, id := range want {
		wantSet[id] = struct{}{}
	}
	for _, id := range got {
		if _, ok := wantSet[id]; !ok {
			t.Fatalf("unexpected id %d in %v (want %v)", id, got, want)
		}
	}
}

func connectionCount(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&n).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	return n
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(raw)
}

func seedLocation(t *testing.T, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		Type:        repolocations.LocationType_REGION,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed location %q: %v", name, err)
	}
	return loc
}

func seedConnection(t *testing.T, fromID, toID repositories.ID, description string, bidirectional bool) *repolocations.Connection {
	t.Helper()
	conn := &repolocations.Connection{
		FromID:        fromID,
		ToID:          toID,
		Description:   description,
		Bidirectional: bidirectional,
		State:         repositories.JSON(`{}`),
		Details:       repositories.JSON(`{}`),
	}
	if err := testRepo.CreateConnection(context.Background(), conn); err != nil {
		t.Fatalf("seed connection %q: %v", description, err)
	}
	return conn
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
		t.Fatalf("find_connections is idempotent and must not journal, got %d rows for %s/%s", n, invokeID, callID)
	}
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
