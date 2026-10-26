package dnd_tavern_scene_test

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
	tavernName            = "The Gilded Tankard"
	closingQuestion       = "Что вы делаете?"
	richTavernDescription = "The Gilded Tankard crouches between two leaning warehouses, its oak sign swinging in the salt wind. Inside, a low fire spits resinous sparks under a soot-blackened beam hung with tin mugs. Fishermen argue over charts at the corner table while a one-eyed bard plucks a battered lute near the stairs. The air smells of fried onions, wet wool, and cheap rum; every floorboard creaks a different warning."
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
		fmt.Fprintf(os.Stderr, "skip dnd_tavern_scene Invoke tests: %v\n", reason)
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
	findTool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
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

func TestInvoke_DnD_TavernScene(t *testing.T) {
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
	if err := testRepo.CreateSubtype(context.Background(), subtype); err != nil {
		t.Fatalf("seed tavern subtype: %v", err)
	}

	tavern, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    new(world.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        tavernName,
		Description: richTavernDescription,
		Details: repositories.JSON(`{
			"atmosphere":"smoky and loud",
			"keeper":"Marta Reed-Eye",
			"specialty":"pepper-crab stew",
			"rumor":"a sealed map is hidden under the third floorboard by the hearth"
		}`),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 360)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	fmt.Printf("=== Seeded tavern ===\nid=%d name=%q subtype=%d\n", tavern.ID, tavern.Name, subtype.ID)

	// Round 1: open the tavern scene.
	round1 := fmt.Sprintf(
		"Отыграй 1 сцену мастера DND по локации id=%d (find_locations: scope_id=%d, depth=0). Опиши сцену через один send_message и закончи словами: %s. Отдельным send_message укажи, почему ты описал сцену именно так и почему некоторые моменты оставил без внимания или упоминания.",
		tavern.ID,
		tavern.ID,
		closingQuestion,
	)
	invoke1 := uuid.NewString()
	fmt.Printf("=== Round 1 Invoke ===\ninvoke_id: %s\nuser: %s\n", invoke1, round1)
	if err := provider.Invoke(ctx, invoke1, providers.Request{
		Args: json.RawMessage(round1),
	}); err != nil {
		t.Fatalf("round 1 Invoke: %v", err)
	}
	msgs1 := assertPlayerAndExplanation(t, invoke1, "round 1")
	if !strings.Contains(msgs1[0].Text, closingQuestion) {
		t.Fatalf("round 1 scene message must contain %q, got %q", closingQuestion, msgs1[0].Text)
	}

	// Round 2: player examines the floorboards.
	round2 := fmt.Sprintf(
		"Игрок говорит: я осматриваю половицы. Отыграй реакцию мастера по локации id=%d (find_locations: scope_id=%d, depth=0) через один send_message. Отдельным send_message укажи, почему ты описал реакцию именно так и почему некоторые моменты оставил без внимания или упоминания.",
		tavern.ID,
		tavern.ID,
	)
	invoke2 := uuid.NewString()
	fmt.Printf("=== Round 2 Invoke ===\ninvoke_id: %s\nuser: %s\n", invoke2, round2)
	if err := provider.Invoke(ctx, invoke2, providers.Request{
		Args: json.RawMessage(round2),
	}); err != nil {
		t.Fatalf("round 2 Invoke: %v", err)
	}
	_ = assertPlayerAndExplanation(t, invoke2, "round 2")
}

// assertPlayerAndExplanation checks completed journal send_message rows and
// at least two saved messages (player-facing + technical explanation).
func assertPlayerAndExplanation(t *testing.T, invokeID, round string) []repomessages.Message {
	t.Helper()

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, "send_message").
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("%s: load journal requests: %v", round, err)
	}
	if len(journaled) < 2 {
		t.Fatalf("%s: expected at least two send_message rows in journal (player + explanation), got %d", round, len(journaled))
	}
	for _, req := range journaled {
		if req.Status != journal.Status_Completed {
			t.Fatalf("%s: journal request %s: want status %s, got %s", round, req.ID, journal.Status_Completed, req.Status)
		}
	}

	var saved []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&saved).Error; err != nil {
		t.Fatalf("%s: load messages: %v", round, err)
	}
	if len(saved) < 2 {
		t.Fatalf("%s: expected at least two messages (player + explanation), got %d", round, len(saved))
	}

	fmt.Printf("=== %s journaled send_message (%d) ===\n", round, len(journaled))
	for _, req := range journaled {
		fmt.Printf("id=%s status=%s\n", req.ID, req.Status)
	}
	fmt.Printf("=== %s saved messages (%d) ===\n", round, len(saved))
	for _, m := range saved {
		fmt.Printf("seq=%d id=%d from=%s text=%q\n", m.SeqNum, m.ID, m.From, m.Text)
	}
	return saved
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
