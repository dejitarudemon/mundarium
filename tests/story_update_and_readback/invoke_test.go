package story_update_and_readback_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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

const (
	tavernName    = "The Copper Fox"
	cellarName    = "Cellar"
	oldKeeper     = "Marta"
	newKeeper     = "Edwin"
	seededDetails = `{"keeper":"Marta","rooms":4,"rumor":"A map is hidden beneath the hearth"}`
	seededDesc    = "A copper-signed tavern by the market square."
)

// Player-facing messages may transliterate Latin keeper names (e.g. Edwin → Эдвин).
var (
	newKeeperAliases = []string{"Edwin", "Эдвин", "Эдвина", "Эдвину", "Эдвином"}
	oldKeeperAliases = []string{"Marta", "Марта", "Марты", "Марте", "Мартой"}
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
		fmt.Fprintf(os.Stderr, "skip story_update_and_readback Invoke tests: %v\n", reason)
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

func TestInvoke_Story_UpdateAndReadback(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

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

	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        tavernName,
		Description: seededDesc,
		Details:     repositories.JSON(seededDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	cellar, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(tavern.ID),
		Type:        repolocations.LocationType_ROOM,
		Name:        cellarName,
		Description: "Cool storage under the tavern.",
		Details:     repositories.JSON(`{"barrels":12}`),
	})
	if err != nil {
		t.Fatalf("seed cellar: %v", err)
	}

	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count before: %v", err)
	}
	beforeMeta := snapshotMeta(t, tavern.ID)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 240)) * time.Second

	// --- Invoke 1: record ownership change in details ---
	invoke1 := uuid.NewString()
	msg1 := fmt.Sprintf(
		`Игрок узнал: таверна %q (id=%d, сейчас в details keeper=%q) недавно сменила владельца на %q. Зафиксируй этот факт в details существующей локации через update_location_details (full overwrite; сохрани rooms и остальные нужные поля, замени keeper). Новую локацию не создавай. Подвал id=%d не трогай. Затем одним send_message сообщи игроку, кто теперь владелец.`,
		tavernName, tavern.ID, oldKeeper, newKeeper, cellar.ID,
	)

	fmt.Printf("=== Seed ===\nsettlement=%d tavern=%d cellar=%d keeper=%q\n",
		settlement.ID, tavern.ID, cellar.ID, oldKeeper)
	fmt.Printf("=== Invoke 1 ===\ninvoke_id: %s\nuser: %s\n", invoke1, msg1)

	invokeCtx1, cancel1 := context.WithTimeout(context.Background(), timeout)
	defer cancel1()
	if err := provider.Invoke(invokeCtx1, invoke1, providers.Request{
		Args: json.RawMessage(msg1),
	}); err != nil {
		t.Fatalf("Invoke 1: %v", err)
	}

	assertNoCreates(t, invoke1)
	assertSendMessage(t, invoke1)

	var locationCountAfter1 int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter1).Error; err != nil {
		t.Fatalf("count after invoke1: %v", err)
	}
	if locationCountAfter1 != locationCountBefore {
		t.Fatalf("invoke1 must not create locations: before=%d after=%d", locationCountBefore, locationCountAfter1)
	}

	got, err := testRepo.GetLocation(ctx, tavern.ID)
	if err != nil {
		t.Fatalf("GetLocation tavern after invoke1: %v", err)
	}
	var details map[string]any
	if err := json.Unmarshal(got.Details, &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	keeper, _ := details["keeper"].(string)
	if keeper != newKeeper {
		t.Fatalf("PostgreSQL keeper after invoke1: want %q, got %#v in %s", newKeeper, details["keeper"], got.Details)
	}
	if keeper == oldKeeper {
		t.Fatalf("old keeper %q must not remain current", oldKeeper)
	}

	afterMeta := snapshotMeta(t, tavern.ID)
	if afterMeta.Name != beforeMeta.Name ||
		afterMeta.Description != beforeMeta.Description ||
		afterMeta.ParentID != beforeMeta.ParentID ||
		afterMeta.Type != beforeMeta.Type ||
		afterMeta.SubtypeID != beforeMeta.SubtypeID {
		t.Fatalf("invoke1 must only change details:\nbefore=%+v\nafter=%+v", beforeMeta, afterMeta)
	}

	cellarGot, err := testRepo.GetLocation(ctx, cellar.ID)
	if err != nil {
		t.Fatalf("GetLocation cellar: %v", err)
	}
	if cellarGot.ParentID == nil || *cellarGot.ParentID != tavern.ID {
		t.Fatalf("cellar must still belong to tavern %d, got parent=%v", tavern.ID, cellarGot.ParentID)
	}

	fmt.Printf("=== After invoke1 details ===\n%s\n", got.Details)

	// --- Invoke 2: fresh session — read back from DB via find ---
	invoke2 := uuid.NewString()
	msg2 := fmt.Sprintf(
		`Игрок спрашивает: кому теперь принадлежит таверна %q в поселении scope_id=%d? Сначала получи актуальные данные через find_locations (query=%q или depth=1 по scope), затем одним send_message ответь текущим владельцем из details. Не опирайся на прошлый разговор — его нет. Не создавай локации.`,
		tavernName, settlement.ID, tavernName,
	)

	fmt.Printf("=== Invoke 2 ===\ninvoke_id: %s\nuser: %s\n", invoke2, msg2)

	invokeCtx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	if err := provider.Invoke(invokeCtx2, invoke2, providers.Request{
		Args: json.RawMessage(msg2),
	}); err != nil {
		t.Fatalf("Invoke 2: %v", err)
	}

	assertNoCreates(t, invoke2)
	text2 := assertSendMessage(t, invoke2)

	var locationCountAfter2 int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter2).Error; err != nil {
		t.Fatalf("count after invoke2: %v", err)
	}
	if locationCountAfter2 != locationCountBefore {
		t.Fatalf("invoke2 must not create locations: before=%d after=%d", locationCountBefore, locationCountAfter2)
	}

	// Re-check DB still has new keeper (canon persisted across invokes).
	got2, err := testRepo.GetLocation(ctx, tavern.ID)
	if err != nil {
		t.Fatalf("GetLocation after invoke2: %v", err)
	}
	var details2 map[string]any
	if err := json.Unmarshal(got2.Details, &details2); err != nil {
		t.Fatalf("unmarshal details2: %v", err)
	}
	if k, _ := details2["keeper"].(string); k != newKeeper {
		t.Fatalf("DB keeper after invoke2: want %q, got %#v", newKeeper, details2["keeper"])
	}

	if !messageMentionsAny(text2, newKeeperAliases) {
		t.Fatalf("invoke2 message must mention current keeper %q (or transliteration), got %q", newKeeper, text2)
	}
	// Old keeper must not be presented as the current owner.
	if presentsOldKeeperAsCurrent(text2) {
		t.Fatalf("invoke2 must not present old keeper %q as current owner, got %q", oldKeeper, text2)
	}

	fmt.Printf("=== Invoke2 player message ===\n%s\n", text2)
	fmt.Printf("=== Canon details ===\n%s\n", got2.Details)
}

func messageMentionsAny(text string, aliases []string) bool {
	for _, alias := range aliases {
		if strings.Contains(text, alias) {
			return true
		}
	}
	return false
}

func presentsOldKeeperAsCurrent(text string) bool {
	if !messageMentionsAny(text, oldKeeperAliases) {
		return false
	}
	lower := strings.ToLower(text)
	// Allow mentioning Marta only as former/previous owner.
	negationHints := []string{
		"бывш", "прежн", "раньше", "ранее", "former", "previous", "used to", "no longer", "wasn't", "was not",
	}
	for _, alias := range oldKeeperAliases {
		negationHints = append(negationHints, "не "+strings.ToLower(alias))
	}
	for _, hint := range negationHints {
		if strings.Contains(lower, hint) {
			return false
		}
	}
	return true
}

func assertNoCreates(t *testing.T, invokeID string) {
	t.Helper()
	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal %s: %v", invokeID, err)
	}
	for _, req := range journaled {
		switch req.Tool {
		case "create_location", "create_subtype":
			t.Fatalf("invoke %s: %s must not be called", invokeID, req.Tool)
		case "update_location_details", "find_locations":
			t.Fatalf("invoke %s: %s is idempotent and must not be journaled", invokeID, req.Tool)
		}
	}
}

func assertSendMessage(t *testing.T, invokeID string) string {
	t.Helper()
	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, "send_message").
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load send_message journal: %v", err)
	}
	if len(journaled) == 0 {
		t.Fatalf("invoke %s: expected send_message journal row", invokeID)
	}
	for _, req := range journaled {
		if req.Status != journal.Status_Completed {
			t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
		}
	}

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("invoke %s: expected player message row", invokeID)
	}
	return messages[len(messages)-1].Text
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
