package create_tavern_test

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
		fmt.Fprintf(os.Stderr, "skip create_tavern Invoke tests: %v\n", reason)
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
		fmt.Fprintf(os.Stderr, "register tool: %v\n", err)
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

func TestInvoke_CreateTavern_RichDescription(t *testing.T) {
	resetDB(t)

	world, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: nil,
		Type:     repolocations.LocationType_WORLD,
		Name:     "SeedWorld",
	})
	if err != nil {
		t.Fatalf("seed world: %v", err)
	}

	subtype := &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest", "sleep", "enclosed space"},
		ExpectedTags: pq.StringArray{"village", "city", "rumors"},
		Description:  "used for safe, paid recreation",
	}
	if _, err := testRepo.CreateSubtype(context.Background(), subtype); err != nil {
		t.Fatalf("seed tavern subtype: %v", err)
	}

	userMessage := fmt.Sprintf(
		"Создай таверну через create_location: parent_id=%d, type=structure, subtype=%d, имя на твой вкус. Дай богатое многопредложенческое описание и несколько details. Обязательно заполни аргумент description минимум тремя предложениями, а details — минимум четырьмя ключами. Не оставляй эти поля пустыми. Другие тулзы не вызывай.",
		world.ID,
		subtype.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nworld_id=%d subtype_id=%d key=%s\n", world.ID, subtype.ID, subtype.Key)
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
		t.Fatalf("load journal requests: %v", err)
	}
	if len(journaled) == 0 {
		t.Fatal("expected at least one create_location row in execution_journal_requests")
	}
	for _, req := range journaled {
		if req.Status != journal.Status_Completed {
			t.Fatalf("journal request %s: want status %s, got %s", req.ID, journal.Status_Completed, req.Status)
		}
	}

	var taverns []repolocations.Location
	if err := testConn.DB().
		Where("parent_id = ? AND type = ?", world.ID, repolocations.LocationType_STRUCTURE).
		Order("id asc").
		Find(&taverns).Error; err != nil {
		t.Fatalf("load taverns: %v", err)
	}
	if len(taverns) == 0 {
		t.Fatal("expected at least one structure location under seeded world")
	}

	for _, loc := range taverns {
		if loc.SubtypeID == nil || *loc.SubtypeID != subtype.ID {
			t.Fatalf("location %d: want subtype_id=%d, got %v", loc.ID, subtype.ID, loc.SubtypeID)
		}
		if len(loc.Description) < 40 {
			t.Fatalf("location %d: want rich description, got %q", loc.ID, loc.Description)
		}
	}

	fmt.Printf("=== Journaled create_location (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("id=%s status=%s\n", req.ID, req.Status)
	}
	fmt.Printf("=== Saved taverns (%d) ===\n", len(taverns))
	for _, loc := range taverns {
		view := struct {
			ID          repositories.ID   `json:"id"`
			ParentID    *repositories.ID  `json:"parent_id"`
			Type        string            `json:"type"`
			SubtypeID   *repositories.ID  `json:"subtype_id"`
			Name        string            `json:"name"`
			Description string            `json:"description"`
			Details     repositories.JSON `json:"Details"`
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
			t.Fatalf("marshal location %d: %v", loc.ID, err)
		}
		fmt.Printf("%s\n", raw)
	}
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE connections, locations, location_subtypes, execution_journal_requests
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
