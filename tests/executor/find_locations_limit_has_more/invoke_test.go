package find_locations_limit_has_more_test

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
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const (
	childCount = 12
	findLimit  = 5
	findDepth  = 1
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// Deterministic tool/executor test: needs Postgres, not LLM Studio.
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

	tool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "find_location.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
		return
	}
	if err := exec.Register(tool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestInvoke_FindLocations_LimitAndHasMore(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	parent, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "LimitParent",
		Description: "parent with many children",
		Details:     repositories.JSON(`{"seed":true}`),
	})
	if err != nil {
		t.Fatalf("seed parent: %v", err)
	}

	childIDs := make([]repositories.ID, 0, childCount)
	for i := 1; i <= childCount; i++ {
		child, err := testRepo.CreateLocation(ctx, &repolocations.Location{
			ParentID:    new(parent.ID),
			Type:        repolocations.LocationType_STRUCTURE,
			Name:        fmt.Sprintf("Child-%02d", i),
			Description: fmt.Sprintf("child location %d", i),
			Details:     repositories.JSON(fmt.Sprintf(`{"n":%d}`, i)),
		})
		if err != nil {
			t.Fatalf("seed child %d: %v", i, err)
		}
		childIDs = append(childIDs, child.ID)
	}

	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count before: %v", err)
	}

	args, err := json.Marshal(map[string]any{
		"scope_id": parent.ID,
		"depth":    findDepth,
		"limit":    findLimit,
	})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}

	invokeID := uuid.NewString()
	raw, err := ex.Execute(ctx, invokeID, tools.Call{
		ID:   "find-limit-1",
		Name: "find_locations",
		Role: types.Role_Master,
		Args: args,
	})
	if err != nil {
		t.Fatalf("Executor.Execute find_locations: %v", err)
	}

	var out struct {
		Scope struct {
			ID   repositories.ID `json:"id"`
			Name string          `json:"name"`
		} `json:"scope"`
		Children map[string][]struct {
			ID   repositories.ID `json:"id"`
			Name string          `json:"name"`
		} `json:"children"`
		HasMore int `json:"has_more"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal tool result: %v\nraw=%s", err, raw)
	}

	if out.Scope.ID != parent.ID || out.Scope.Name != parent.Name {
		t.Fatalf("scope: want id=%d name=%q, got id=%d name=%q", parent.ID, parent.Name, out.Scope.ID, out.Scope.Name)
	}

	seen := map[repositories.ID]bool{}
	descendantCount := 0
	for depth, locs := range out.Children {
		for _, loc := range locs {
			if seen[loc.ID] {
				t.Fatalf("duplicate id=%d in children (depth key %s)", loc.ID, depth)
			}
			seen[loc.ID] = true
			descendantCount++
			if loc.ID == parent.ID {
				t.Fatalf("scope id must not appear among children")
			}
		}
	}
	if descendantCount > findLimit {
		t.Fatalf("want at most %d descendants, got %d", findLimit, descendantCount)
	}
	if descendantCount != findLimit {
		t.Fatalf("want exactly %d descendants with limit=%d and %d children, got %d", findLimit, findLimit, childCount, descendantCount)
	}

	wantHasMore := childCount - findLimit
	if out.HasMore != wantHasMore {
		t.Fatalf("has_more: want %d, got %d", wantHasMore, out.HasMore)
	}

	for id := range seen {
		ok := false
		for _, childID := range childIDs {
			if id == childID {
				ok = true
				break
			}
		}
		if !ok {
			t.Fatalf("result id=%d is outside seeded children", id)
		}
	}

	var locationCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter).Error; err != nil {
		t.Fatalf("count after: %v", err)
	}
	if locationCountAfter != locationCountBefore {
		t.Fatalf("DB must not change: before=%d after=%d", locationCountBefore, locationCountAfter)
	}

	// Idempotent find must not journal.
	var journalCount int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ?", invokeID).
		Count(&journalCount).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if journalCount != 0 {
		t.Fatalf("find_locations must not create journal rows, got %d", journalCount)
	}

	fmt.Printf("=== Tool result ===\n%s\n", raw)
	fmt.Printf("=== Asserted ===\nscope=%d descendants=%d has_more=%d\n", out.Scope.ID, descendantCount, out.HasMore)
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
