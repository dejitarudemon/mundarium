package update_location_details_full_overwrite_test

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
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const (
	tavernName     = "The Copper Fox"
	seededDetails  = `{"keeper":"Marta","rumor":"A map is hidden beneath the hearth","rooms":4}`
	wantDetailsRaw = `{"keeper":"Edwin","rooms":6}`
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
		fmt.Fprintf(os.Stderr, "skip update_location_details_full_overwrite Invoke tests: %v\n", reason)
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

	locGenerator := func(db *gorm.DB) *repolocations.LocationRepository {
		repo, err := repolocations.NewLocationRepository(db, 5, log)
		if err != nil {
			panic(err)
		}
		return repo
	}
	msgGenerator := func(db *gorm.DB) *repomessages.MessagesRepository {
		repo, err := repomessages.NewMessagesRepository(db, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	locSvc, err := svclocations.NewService(locGenerator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new locations service: %v\n", err)
		return
	}
	msgSvc, err := svcmessages.NewService(msgGenerator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new messages service: %v\n", err)
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
	role := []types.Role{types.Role_Master}

	findTool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
		return
	}
	updateTool, err := toolslocations.NewUpdateLocationDetailsTool(
		filepath.Join(root, "definitions", "update_location_details.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_location_details tool: %v\n", err)
		return
	}
	sendTool, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"),
		msgSvc,
		nil,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message tool: %v\n", err)
		return
	}
	if err := ex.Register(findTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}
	if err := ex.Register(updateTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register update_location_details: %v\n", err)
		return
	}
	if err := ex.Register(sendTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register send_message: %v\n", err)
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
	testRepo = locGenerator(conn.DB())
	provider = p
	code = m.Run()
}

func TestInvoke_UpdateLocationDetails_FullOverwrite(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	subtype, err := testRepo.CreateSubtype(ctx, &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"},
		ExpectedTags: pq.StringArray{"village"},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("seed subtype: %v", err)
	}

	settlement, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "Ashford",
		Description: "market town",
		Details:     repositories.JSON(`{"size":"medium"}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	const seededDescription = "A copper-signed tavern by the market square."
	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        tavernName,
		Description: seededDescription,
		Details:     repositories.JSON(seededDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}

	before := snapshotMeta(t, tavern.ID)

	userMessage := fmt.Sprintf(
		`У локации %q (id=%d) полностью замени details через update_location_details (full overwrite, не merge) на объект ровно %s. Поле rumor должно исчезнуть. Затем одним send_message подтверди id и итоговые details. Другие поля и локации не меняй и не создавай.`,
		tavernName, tavern.ID, wantDetailsRaw,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\ntavern=%d details=%s\n", tavern.ID, seededDetails)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(invokeCtx, invokeID, providers.Request{
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

	sendRows := 0
	for _, req := range journaled {
		switch req.Tool {
		case "update_location_details", "find_locations":
			t.Fatalf("%s is idempotent and must not be journaled, got %#v", req.Tool, req)
		case "create_location", "create_subtype":
			t.Fatalf("%s must not be called, got journal %#v", req.Tool, req)
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendRows == 0 {
		t.Fatal("expected at least one send_message journal row (update_location_details itself is not journaled)")
	}

	got, err := testRepo.GetLocation(ctx, tavern.ID)
	if err != nil {
		t.Fatalf("GetLocation tavern: %v", err)
	}

	var details map[string]any
	if err := json.Unmarshal(got.Details, &details); err != nil {
		t.Fatalf("unmarshal details %s: %v", got.Details, err)
	}
	if len(details) != 2 {
		t.Fatalf("want exactly 2 detail keys after full overwrite, got %d: %s", len(details), got.Details)
	}
	if _, ok := details["rumor"]; ok {
		t.Fatalf("rumor must be removed (full overwrite, not merge), got %s", got.Details)
	}
	keeper, ok := details["keeper"].(string)
	if !ok || keeper != "Edwin" {
		t.Fatalf("keeper: want Edwin, got %#v in %s", details["keeper"], got.Details)
	}
	rooms, ok := details["rooms"].(float64) // JSON numbers -> float64
	if !ok || rooms != 6 {
		t.Fatalf("rooms: want 6, got %#v in %s", details["rooms"], got.Details)
	}

	after := snapshotMeta(t, tavern.ID)
	if after.Name != before.Name ||
		after.Description != before.Description ||
		after.ParentID != before.ParentID ||
		after.Type != before.Type ||
		after.SubtypeID != before.SubtypeID {
		t.Fatalf("non-details fields must not change:\nbefore=%+v\nafter=%+v", before, after)
	}

	var locationCount int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCount).Error; err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if locationCount != 2 {
		t.Fatalf("want settlement+tavern only (2), got %d", locationCount)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Details after ===\n%s\n", got.Details)
}

type locationMeta struct {
	Name        string
	Description string
	ParentID    repositories.ID
	Type        repolocations.LocationType
	SubtypeID   repositories.ID
}

func snapshotMeta(t *testing.T, id repositories.ID) locationMeta {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	meta := locationMeta{
		Name:        got.Name,
		Description: got.Description,
		Type:        got.Type,
	}
	if got.ParentID != nil {
		meta.ParentID = *got.ParentID
	}
	if got.SubtypeID != nil {
		meta.SubtypeID = *got.SubtypeID
	}
	return meta
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
