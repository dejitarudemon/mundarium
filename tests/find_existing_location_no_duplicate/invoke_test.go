package find_existing_location_no_duplicate_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

const copperFoxName = "The Copper Fox"

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip find_existing_location_no_duplicate Invoke tests: %v\n", reason)
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
	findLocations, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
		return
	}
	createLocation, err := toolslocations.NewCreateLocationTool(
		filepath.Join(root, "definitions", "create_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_location tool: %v\n", err)
		return
	}
	sendMessage, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"),
		msgSvc,
		nil,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message tool: %v\n", err)
		return
	}

	role := []types.Role{types.Role_Master}
	if err := ex.Register(findLocations, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}
	if err := ex.Register(createLocation, role); err != nil {
		fmt.Fprintf(os.Stderr, "register create_location: %v\n", err)
		return
	}
	if err := ex.Register(sendMessage, role); err != nil {
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

func TestInvoke_FindExistingLocation_NoDuplicate(t *testing.T) {
	resetDB(t)

	settlement, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "SeedSettlement",
		Description: "market town",
		Details:     repositories.JSON(`{"size":"medium"}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	subtype, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"},
		ExpectedTags: pq.StringArray{"village"},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("seed subtype: %v", err)
	}

	const (
		seededDescription = "A copper-signed tavern by the market square."
		seededDetails     = `{"keeper":"Marta","rooms":4}`
	)
	tavern, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        copperFoxName,
		Description: seededDescription,
		Details:     repositories.JSON(seededDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}

	before := snapshotLocation(t, tavern.ID)
	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count locations before: %v", err)
	}

	userMessage := fmt.Sprintf(
		`В поселении scope_id=%d (find_locations: depth=1, query=%q) проверь, есть ли таверна с именем %q. Если уже есть — не создавай дубликат; одним send_message сообщи игроку её id и имя. create_location вызывай только если локация отсутствует.`,
		settlement.ID,
		copperFoxName,
		copperFoxName,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nsettlement=%d tavern=%d name=%q\n", settlement.ID, tavern.ID, copperFoxName)
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

	var createLocationRows, sendMessageRows int
	for _, req := range journaled {
		switch req.Tool {
		case "create_location":
			createLocationRows++
		case "send_message":
			sendMessageRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		case "find_locations":
			t.Fatalf("find_locations is idempotent and must not be journaled, got %#v", req)
		}
	}
	if createLocationRows != 0 {
		t.Fatalf("create_location must not be called, got %d journal rows", createLocationRows)
	}
	if sendMessageRows == 0 {
		t.Fatal("expected at least one send_message journal row")
	}

	var locationCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter).Error; err != nil {
		t.Fatalf("count locations after: %v", err)
	}
	if locationCountAfter != locationCountBefore {
		t.Fatalf("location count must not increase: before=%d after=%d", locationCountBefore, locationCountAfter)
	}

	after := snapshotLocation(t, tavern.ID)
	if after != before {
		t.Fatalf("existing tavern mutated:\nbefore=%+v\nafter=%+v", before, after)
	}

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected send_message text referencing the found tavern")
	}
	text := messages[len(messages)-1].Text
	idStr := strconv.FormatUint(uint64(tavern.ID), 10)
	if !strings.Contains(text, idStr) || !strings.Contains(text, copperFoxName) {
		t.Fatalf("message must include tavern id=%s and name %q, got %q", idStr, copperFoxName, text)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Player message ===\n%s\n", text)
	fmt.Printf("=== Unchanged tavern ===\n%+v\n", after)
}

type locationSnap struct {
	Name        string
	Description string
	Details     string
	ParentID    repositories.ID
	Type        repolocations.LocationType
	SubtypeID   repositories.ID
}

func snapshotLocation(t *testing.T, id repositories.ID) locationSnap {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	snap := locationSnap{
		Name:        got.Name,
		Description: got.Description,
		Details:     string(got.Details),
		Type:        got.Type,
	}
	if got.ParentID != nil {
		snap.ParentID = *got.ParentID
	}
	if got.SubtypeID != nil {
		snap.SubtypeID = *got.SubtypeID
	}
	return snap
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
