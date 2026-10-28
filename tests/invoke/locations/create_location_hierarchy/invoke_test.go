package create_location_hierarchy_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/providers"
	openai "github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const worldName = "Test World"

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip create_location_hierarchy Invoke tests: %v\n", reason)
		os.Exit(0)
	}

	prompt, err := testutil.LoadSystemPrompt(filepath.Join(packageDir(), "system_prompt.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load system prompt: %v\n", err)
		os.Exit(1)
	}

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

	ex, err := executor.NewExecutor(j, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new executor: %v\n", err)
		return
	}

	tool, err := toolslocations.NewCreateLocationTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "create_location.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_location tool: %v\n", err)
		return
	}
	if err := ex.Register(tool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register create_location: %v\n", err)
		return
	}

	p, err := openai.NewOpenAICompatibleProvider(
		ex, url, model, apiKey, prompt, types.Role_Master, maxIters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	provider = p
	code = m.Run()
}

func TestInvoke_CreateLocationHierarchy(t *testing.T) {
	resetDB(t)

	world, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_WORLD,
		Name:        worldName,
		Description: "seeded root world",
		Details:     repositories.JSON(`{"seed":true}`),
	})
	if err != nil {
		t.Fatalf("seed world: %v", err)
	}

	userMessage := fmt.Sprintf(
		`Под корнем parent_id=%d (%q) создай цепочку через create_location без subtype:
1) поселение type=settlement;
2) внутри него таверну type=structure (parent_id = id поселения из предыдущего ответа);
3) внутри таверны подвал type=room (parent_id = id таверны).
Для каждой локации заполни description и details. Другие тулзы не вызывай. Не создавай дополнительные корневые локации.`,
		world.ID,
		worldName,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 240)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nworld_id=%d name=%q\n", world.ID, worldName)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, "create_location").
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	if len(journaled) != 3 {
		t.Fatalf("expected exactly 3 create_location journal rows, got %d", len(journaled))
	}
	for _, req := range journaled {
		if req.Status != journal.Status_Completed {
			t.Fatalf("create_location %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
		}
	}

	var all []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("load locations: %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("want world + 3 created locations (4 total), got %d: %#v", len(all), names(all))
	}

	var roots []repolocations.Location
	if err := testConn.DB().Where("parent_id IS NULL").Find(&roots).Error; err != nil {
		t.Fatalf("load roots: %v", err)
	}
	if len(roots) != 1 || roots[0].ID != world.ID {
		t.Fatalf("want single root world id=%d, got %#v", world.ID, roots)
	}

	settlement := mustOneChild(t, world.ID, repolocations.LocationType_SETTLEMENT)
	tavern := mustOneChild(t, settlement.ID, repolocations.LocationType_STRUCTURE)
	cellar := mustOneChild(t, tavern.ID, repolocations.LocationType_ROOM)

	for _, loc := range []repolocations.Location{settlement, tavern, cellar} {
		if loc.Description == "" {
			t.Fatalf("location %d (%s): description required", loc.ID, loc.Name)
		}
		if len(loc.Details) == 0 || string(loc.Details) == "null" {
			t.Fatalf("location %d (%s): details required, got %s", loc.ID, loc.Name, loc.Details)
		}
		if loc.SubtypeID != nil {
			t.Fatalf("location %d: expected nil subtype, got %v", loc.ID, *loc.SubtypeID)
		}
	}

	// Deterministic find: depth=3 from world should surface settlement, tavern, cellar.
	found, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: world.ID,
		Depth:   3,
	})
	if err != nil {
		t.Fatalf("FindLocations depth=3: %v", err)
	}
	if found.Scope.ID != world.ID {
		t.Fatalf("find scope: want %d, got %d", world.ID, found.Scope.ID)
	}
	byID := map[repositories.ID]repolocations.Location{}
	for _, level := range found.Children {
		for _, loc := range level {
			byID[loc.ID] = loc
		}
	}
	for _, want := range []repolocations.Location{settlement, tavern, cellar} {
		if _, ok := byID[want.ID]; !ok {
			t.Fatalf("FindLocations depth=3 missing id=%d name=%q; children=%#v", want.ID, want.Name, found.Children)
		}
	}

	fmt.Printf("=== Journaled create_location (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("id=%s status=%s\n", req.ID, req.Status)
	}
	dumpLocation(t, "World", *world)
	dumpLocation(t, "Settlement", settlement)
	dumpLocation(t, "Tavern", tavern)
	dumpLocation(t, "Cellar", cellar)
}

func mustOneChild(t *testing.T, parentID repositories.ID, wantType repolocations.LocationType) repolocations.Location {
	t.Helper()
	var children []repolocations.Location
	if err := testConn.DB().
		Where("parent_id = ?", parentID).
		Order("id asc").
		Find(&children).Error; err != nil {
		t.Fatalf("load children of %d: %v", parentID, err)
	}
	if len(children) != 1 {
		t.Fatalf("parent %d: want exactly one child, got %d (%v)", parentID, len(children), names(children))
	}
	child := children[0]
	if child.Type != wantType {
		t.Fatalf("child of %d: want type %s, got %s (%q)", parentID, wantType.String(), child.Type.String(), child.Name)
	}
	if child.ParentID == nil || *child.ParentID != parentID {
		t.Fatalf("child %d: want parent_id=%d, got %v", child.ID, parentID, child.ParentID)
	}
	return child
}

func names(locs []repolocations.Location) []string {
	out := make([]string, len(locs))
	for i, loc := range locs {
		out[i] = fmt.Sprintf("%d:%s", loc.ID, loc.Name)
	}
	return out
}

func dumpLocation(t *testing.T, label string, loc repolocations.Location) {
	t.Helper()
	view := struct {
		ID          repositories.ID   `json:"id"`
		ParentID    *repositories.ID  `json:"parent_id"`
		Type        string            `json:"type"`
		SubtypeID   *repositories.ID  `json:"subtype_id"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Details     repositories.JSON `json:"details"`
	}{
		ID:          loc.ID,
		ParentID:    loc.ParentID,
		Type:        loc.Type.String(),
		SubtypeID:   loc.SubtypeID,
		Name:        loc.Name,
		Description: loc.Description,
		Details:     loc.Details,
	}
	raw, err := json.MarshalIndent(view, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", label, err)
	}
	fmt.Printf("=== %s ===\n%s\n", label, raw)
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

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}
