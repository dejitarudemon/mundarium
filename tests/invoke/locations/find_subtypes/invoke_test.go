package find_subtypes_test

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

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip find_subtypes Invoke tests: %v\n", reason)
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
	findTool, err := toolslocations.NewFindSubtypesTool(
		filepath.Join(root, "definitions", "find_subtypes.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_subtypes tool: %v\n", err)
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
	if err := ex.Register(findTool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register find_subtypes: %v\n", err)
		return
	}
	if err := ex.Register(sendTool, []types.Role{types.Role_Master}); err != nil {
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

func TestInvoke_FindSubtypes_CombinedFilters(t *testing.T) {
	resetDB(t)

	tavern := seedSubtype(t, "tavern", repolocations.LocationType_STRUCTURE, "inn")
	inn := seedSubtype(t, "inn", repolocations.LocationType_STRUCTURE, "lodging")
	wizard := seedSubtype(t, "wizard_tower", repolocations.LocationType_STRUCTURE, "arcane")
	tavernRoom := seedSubtype(t, "tavern_room", repolocations.LocationType_ROOM, "guest room")

	before := snapshotSubtypes(t)

	userMessage := `Вызови find_subtypes с keys=["TAV","INN"] и type="structure". Затем одним send_message сообщи игроку id и key каждого найденного подтипа. Другие тулзы не вызывай.`

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seeded subtypes ===\ntavern=%d inn=%d wizard_tower=%d tavern_room=%d\n",
		tavern.ID, inn.ID, wizard.ID, tavernRoom.ID)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	after := snapshotSubtypes(t)
	if len(after) != len(before) {
		t.Fatalf("DB mutated: subtype count before=%d after=%d", len(before), len(after))
	}
	for id, row := range before {
		got, ok := after[id]
		if !ok || got.Key != row.Key || got.Type != row.Type || got.Description != row.Description {
			t.Fatalf("subtype %d changed: before=%+v after=%+v", id, row, got)
		}
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	for _, req := range journaled {
		if req.Tool == "find_subtypes" {
			t.Fatalf("find_subtypes is idempotent and must not create journal rows, got %#v", req)
		}
	}
	var sendCount int
	for _, req := range journaled {
		if req.Tool == "send_message" {
			sendCount++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendCount == 0 {
		t.Fatal("expected at least one journaled send_message")
	}

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected send_message text with found subtype ids")
	}

	text := messages[len(messages)-1].Text
	fmt.Printf("=== Player message ===\n%s\n", text)
	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}

	tavernID := strconv.FormatUint(uint64(tavern.ID), 10)
	innID := strconv.FormatUint(uint64(inn.ID), 10)
	wizardID := strconv.FormatUint(uint64(wizard.ID), 10)
	roomID := strconv.FormatUint(uint64(tavernRoom.ID), 10)

	if !strings.Contains(text, tavernID) || !strings.Contains(text, innID) {
		t.Fatalf("message must include tavern id=%s and inn id=%s, got %q", tavernID, innID, text)
	}
	if strings.Contains(text, wizardID) {
		t.Fatalf("message must not include wizard_tower id=%s (type filter / key OR), got %q", wizardID, text)
	}
	if strings.Contains(text, roomID) {
		t.Fatalf("message must not include tavern_room id=%s (type=structure filter), got %q", roomID, text)
	}
}

func seedSubtype(t *testing.T, key string, locType repolocations.LocationType, description string) *repolocations.LocationSubtype {
	t.Helper()
	created, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key:          key,
		Type:         locType,
		DefiningTags: pq.StringArray{"tag-a"},
		ExpectedTags: pq.StringArray{"tag-b"},
		Description:  description,
	})
	if err != nil {
		t.Fatalf("seed subtype %q: %v", key, err)
	}
	return created
}

type subtypeSnap struct {
	Key         string
	Type        repolocations.LocationType
	Description string
}

func snapshotSubtypes(t *testing.T) map[uint64]subtypeSnap {
	t.Helper()
	var rows []repolocations.LocationSubtype
	if err := testConn.DB().Order("id").Find(&rows).Error; err != nil {
		t.Fatalf("snapshot subtypes: %v", err)
	}
	out := make(map[uint64]subtypeSnap, len(rows))
	for _, row := range rows {
		out[uint64(row.ID)] = subtypeSnap{
			Key:         row.Key,
			Type:        row.Type,
			Description: row.Description,
		}
	}
	return out
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
