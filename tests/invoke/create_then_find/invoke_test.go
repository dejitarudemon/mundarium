package create_then_find_test

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

const userMessage = "Создай одну корневую локацию типа world с именем CreatedThenFound через create_location без subtype. Затем найди её через find_locations (scope_id = id созданной локации, depth=0). Другие тулзы не вызывай."

var (
	testConn  *pgstorage.Connection
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip create_then_find Invoke tests: %v\n", reason)
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
	createTool, err := toolslocations.NewCreateLocationTool(
		filepath.Join(root, "definitions", "create_location.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_location tool: %v\n", err)
		return
	}
	findTool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		svc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
		return
	}
	if err := ex.Register(createTool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register create_location: %v\n", err)
		return
	}
	if err := ex.Register(findTool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
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
	provider = p
	code = m.Run()
}

func TestInvoke_CreateLocation_ThenFind(t *testing.T) {
	resetDB(t)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var createdCalls []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, "create_location").
		Order("created_at asc").
		Find(&createdCalls).Error; err != nil {
		t.Fatalf("load create_location journal: %v", err)
	}
	if len(createdCalls) == 0 {
		t.Fatal("expected at least one create_location row in execution_journal_requests")
	}
	for _, req := range createdCalls {
		if req.Status != journal.Status_Completed {
			t.Fatalf("create_location %s: want status %s, got %s", req.ID, journal.Status_Completed, req.Status)
		}
	}

	// find_locations is idempotent and is not written to the execution journal.
	var findCalls int64
	if err := testConn.DB().
		Model(&journal.Request{}).
		Where("group_id = ? AND tool = ?", invokeID, "find_locations").
		Count(&findCalls).Error; err != nil {
		t.Fatalf("count find_locations journal: %v", err)
	}
	if findCalls != 0 {
		t.Fatalf("find_locations must not be journaled (idempotent), got %d rows", findCalls)
	}

	var locations []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&locations).Error; err != nil {
		t.Fatalf("load locations: %v", err)
	}
	if len(locations) == 0 {
		t.Fatal("expected at least one location row after create_location")
	}

	fmt.Printf("=== Journaled create_location (%d) ===\n", len(createdCalls))
	for _, req := range createdCalls {
		fmt.Printf("id=%s status=%s result=%+v\n", req.ID, req.Status, req.Result)
	}
	fmt.Printf("=== Saved locations (%d) ===\n", len(locations))
	for _, loc := range locations {
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
