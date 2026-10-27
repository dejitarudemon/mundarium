package move_location_cycle_rejected_test

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
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const (
	nameA = "LocA"
	nameB = "LocB"
	nameC = "LocC"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip move_location_cycle_rejected Invoke tests: %v\n", reason)
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

	exec, err := executor.NewExecutor(j, log)
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
	moveTool, err := toolslocations.NewMoveLocationTool(
		filepath.Join(root, "definitions", "move_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new move_location tool: %v\n", err)
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
	if err := exec.Register(findTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}
	if err := exec.Register(moveTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register move_location: %v\n", err)
		return
	}
	if err := exec.Register(sendTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register send_message: %v\n", err)
		return
	}

	p, err := openai.NewOpenAICompatibleProvider(
		exec, url, model, apiKey, prompt, types.Role_Master, maxIters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	testRepo = locGenerator(conn.DB())
	provider = p
	ex = exec
	code = m.Run()
}

func TestInvoke_MoveLocation_CycleRejected(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	locA, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_REGION,
		Name:        nameA,
		Description: "root of chain",
		Details:     repositories.JSON(`{"n":"A"}`),
	})
	if err != nil {
		t.Fatalf("seed A: %v", err)
	}
	locB, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(locA.ID),
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        nameB,
		Description: "child of A",
		Details:     repositories.JSON(`{"n":"B"}`),
	})
	if err != nil {
		t.Fatalf("seed B: %v", err)
	}
	locC, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(locB.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		Name:        nameC,
		Description: "child of B",
		Details:     repositories.JSON(`{"n":"C"}`),
	})
	if err != nil {
		t.Fatalf("seed C: %v", err)
	}

	before := dumpHierarchy(t)
	assertChain(t, locA.ID, locB.ID, locC.ID)

	// Deterministic guarantee: moving A under C must be rejected (would cycle A→C→B→A).
	cycleArgs, err := json.Marshal(map[string]any{
		"id":        locA.ID,
		"parent_id": locC.ID,
	})
	if err != nil {
		t.Fatalf("marshal cycle args: %v", err)
	}
	raw, err := ex.Execute(ctx, uuid.NewString(), tools.Call{
		ID:   "move-cycle-1",
		Name: "move_location",
		Role: types.Role_Master,
		Args: cycleArgs,
	})
	if err == nil {
		t.Fatalf("move A under C must fail (cycle), got success raw=%s", raw)
	}
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "cycle") {
		t.Fatalf("cycle error must mention cycle, got: %v", err)
	}
	if len(raw) != 0 {
		t.Fatalf("failed move must not return a successful payload, got %s", raw)
	}

	afterExec := dumpHierarchy(t)
	if afterExec != before {
		t.Fatalf("hierarchy mutated after rejected Executor move:\nbefore=%s\nafter=%s", before, afterExec)
	}
	assertChain(t, locA.ID, locB.ID, locC.ID)

	fmt.Printf("=== Seed ===\nA=%d B=%d C=%d\n", locA.ID, locB.ID, locC.ID)
	fmt.Printf("=== Executor cycle error ===\n%v\n", err)

	// Invoke: model should attempt (or refuse) and must not claim a successful reparent.
	userMessage := fmt.Sprintf(
		`Попробуй переместить локацию %q (id=%d) внутрь %q (id=%d) через move_location. Если инструмент вернёт ошибку — честно сообщи об отказе. Не выдумывай успех. Затем одним send_message опиши итог игроку.`,
		nameA, locA.ID, nameC, locC.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
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
		case "move_location", "find_locations":
			t.Fatalf("%s is idempotent and must not be journaled, got %#v", req.Tool, req)
		case "create_location", "create_subtype":
			t.Fatalf("%s must not be called, got %#v", req.Tool, req)
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendRows == 0 {
		t.Fatal("expected send_message with honest outcome")
	}

	afterInvoke := dumpHierarchy(t)
	if afterInvoke != before {
		t.Fatalf("hierarchy mutated after Invoke:\nbefore=%s\nafter=%s", before, afterInvoke)
	}
	assertChain(t, locA.ID, locB.ID, locC.ID)

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected player message")
	}
	text := messages[len(messages)-1].Text
	if claimsSuccessfulMove(text) {
		t.Fatalf("model must not claim a successful move that did not happen, got %q", text)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Player message ===\n%s\n", text)
	fmt.Printf("=== Hierarchy unchanged ===\n%s\n", afterInvoke)
}

func claimsSuccessfulMove(text string) bool {
	lower := strings.ToLower(text)
	// Failure / refusal wording is fine even if "перемещ" appears.
	failureHints := []string{
		"не удалось", "не получилось", "нельзя", "запрещ", "отклон", "ошибк",
		"цикл", "cycle", "fail", "unable", "cannot", "can't", "could not",
		"отказ", "невозможн", "rejected", "invalid",
	}
	for _, hint := range failureHints {
		if strings.Contains(lower, hint) {
			return false
		}
	}
	successHints := []string{
		"успешн", "перемещена", "перемещено", "переместил", "переместила",
		"moved successfully", "successfully moved", "теперь внутри", "теперь находится в",
		"parent_id стал", "parent_id теперь",
	}
	for _, hint := range successHints {
		if strings.Contains(lower, hint) {
			return true
		}
	}
	return false
}

func assertChain(t *testing.T, idA, idB, idC repositories.ID) {
	t.Helper()
	a, err := testRepo.GetLocation(context.Background(), idA)
	if err != nil {
		t.Fatalf("GetLocation A: %v", err)
	}
	b, err := testRepo.GetLocation(context.Background(), idB)
	if err != nil {
		t.Fatalf("GetLocation B: %v", err)
	}
	c, err := testRepo.GetLocation(context.Background(), idC)
	if err != nil {
		t.Fatalf("GetLocation C: %v", err)
	}
	if a.ParentID != nil {
		t.Fatalf("A must remain root, got parent=%v", *a.ParentID)
	}
	if b.ParentID == nil || *b.ParentID != idA {
		t.Fatalf("B parent: want %d, got %v", idA, b.ParentID)
	}
	if c.ParentID == nil || *c.ParentID != idB {
		t.Fatalf("C parent: want %d, got %v", idB, c.ParentID)
	}
}

func dumpHierarchy(t *testing.T) string {
	t.Helper()
	var all []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("dump locations: %v", err)
	}
	type row struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
		Name     string           `json:"name"`
	}
	rows := make([]row, 0, len(all))
	for _, loc := range all {
		rows = append(rows, row{ID: loc.ID, ParentID: loc.ParentID, Name: loc.Name})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal dump: %v", err)
	}
	return string(raw)
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
