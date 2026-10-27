package create_location_test

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

const userMessage = "Создай одну корневую локацию типа world с именем TestWorld через create_location с произвольным описанием и деталями. Без subtype_id и без subtype. Другие тулзы не вызывай."

var (
	testConn  *pgstorage.Connection
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip create_location Invoke tests: %v\n", reason)
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
	provider = p
	code = m.Run()
}

func TestInvoke_CreateLocation_NoSubtype(t *testing.T) {
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

	var locations []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&locations).Error; err != nil {
		t.Fatalf("load locations: %v", err)
	}
	if len(locations) == 0 {
		t.Fatal("expected at least one location row")
	}
	for _, loc := range locations {
		if loc.SubtypeID != nil {
			t.Fatalf("location %d: want nil subtype_id, got %v", loc.ID, *loc.SubtypeID)
		}
	}

	fmt.Printf("=== Journaled tool calls (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("id=%s tool=%s status=%s\n", req.ID, req.Tool, req.Status)
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
