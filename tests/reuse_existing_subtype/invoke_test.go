package reuse_existing_subtype_test

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
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip reuse_existing_subtype Invoke tests: %v\n", reason)
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

	root := testutil.RepoRoot()
	findSubtypes, err := toolslocations.NewFindSubtypesTool(
		filepath.Join(root, "definitions", "find_subtypes.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_subtypes tool: %v\n", err)
		return
	}
	createSubtype, err := toolslocations.NewCreateSubtypeTool(
		filepath.Join(root, "definitions", "create_subtype.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_subtype tool: %v\n", err)
		return
	}
	createLocation, err := toolslocations.NewCreateLocationTool(
		filepath.Join(root, "definitions", "create_location.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_location tool: %v\n", err)
		return
	}

	role := []types.Role{types.Role_Master}
	if err := ex.Register(findSubtypes, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_subtypes: %v\n", err)
		return
	}
	if err := ex.Register(createSubtype, role); err != nil {
		fmt.Fprintf(os.Stderr, "register create_subtype: %v\n", err)
		return
	}
	if err := ex.Register(createLocation, role); err != nil {
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

func TestInvoke_ReuseExistingSubtype(t *testing.T) {
	resetDB(t)

	world, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "SeedWorld",
	})
	if err != nil {
		t.Fatalf("seed world: %v", err)
	}
	settlement, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    new(world.ID),
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "SeedSettlement",
		Description: "quiet riverside town",
		Details:     repositories.JSON(`{"size":"small"}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}
	tavernSubtype, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest", "sleep"},
		ExpectedTags: pq.StringArray{"village", "rumors"},
		Description:  "used for safe, paid recreation",
	})
	if err != nil {
		t.Fatalf("seed tavern subtype: %v", err)
	}

	var subtypeCountBefore int64
	if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&subtypeCountBefore).Error; err != nil {
		t.Fatalf("count subtypes before: %v", err)
	}
	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count locations before: %v", err)
	}

	// Do not pass subtype id — model must discover it via find_subtypes.
	userMessage := fmt.Sprintf(
		`В поселении parent_id=%d создай новую таверну через create_location (type=structure). Сначала найди подходящий подтип через find_subtypes (keys=["tavern"] или type="structure"). Переиспользуй существующий subtype id. Не вызывай create_subtype. Заполни description и details.`,
		settlement.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nworld=%d settlement=%d tavern_subtype=%d\n", world.ID, settlement.ID, tavernSubtype.ID)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}

	var createLocationRows, createSubtypeRows int
	for _, req := range journaled {
		switch req.Tool {
		case "create_location":
			createLocationRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("create_location %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		case "create_subtype":
			createSubtypeRows++
		case "find_subtypes":
			t.Fatalf("find_subtypes is idempotent and must not be journaled, got %#v", req)
		}
	}
	if createSubtypeRows != 0 {
		t.Fatalf("create_subtype must not be called, got %d journal rows", createSubtypeRows)
	}
	if createLocationRows != 1 {
		t.Fatalf("expected exactly one create_location journal row, got %d", createLocationRows)
	}

	var subtypeCountAfter int64
	if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&subtypeCountAfter).Error; err != nil {
		t.Fatalf("count subtypes after: %v", err)
	}
	if subtypeCountAfter != subtypeCountBefore {
		t.Fatalf("subtype count changed: before=%d after=%d", subtypeCountBefore, subtypeCountAfter)
	}

	var locationCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter).Error; err != nil {
		t.Fatalf("count locations after: %v", err)
	}
	if locationCountAfter != locationCountBefore+1 {
		t.Fatalf("want exactly one new location, before=%d after=%d", locationCountBefore, locationCountAfter)
	}

	var created []repolocations.Location
	if err := testConn.DB().
		Where("parent_id = ?", settlement.ID).
		Order("id asc").
		Find(&created).Error; err != nil {
		t.Fatalf("load settlement children: %v", err)
	}
	if len(created) != 1 {
		t.Fatalf("want exactly one child under settlement %d, got %d: %#v", settlement.ID, len(created), created)
	}
	loc := created[0]
	if loc.ParentID == nil || *loc.ParentID != settlement.ID {
		t.Fatalf("want parent_id=%d, got %v", settlement.ID, loc.ParentID)
	}
	if loc.SubtypeID == nil || *loc.SubtypeID != tavernSubtype.ID {
		t.Fatalf("want subtype_id=%d (existing tavern), got %v — model must reuse find_subtypes result", tavernSubtype.ID, loc.SubtypeID)
	}
	if loc.Type != repolocations.LocationType_STRUCTURE {
		t.Fatalf("want type structure, got %s", loc.Type.String())
	}
	if loc.Description == "" {
		t.Fatalf("location %d: description must be saved", loc.ID)
	}
	if len(loc.Details) == 0 || string(loc.Details) == "null" {
		t.Fatalf("location %d: details must be saved, got %s", loc.ID, loc.Details)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Created tavern ===\nid=%d parent=%d subtype=%d name=%q desc_len=%d details=%s\n",
		loc.ID, settlement.ID, tavernSubtype.ID, loc.Name, len(loc.Description), loc.Details)
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
