package connection_state_and_readback_test

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
	"sync"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/execution"
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
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer

	toolSeqMu sync.Mutex
	toolSeq   []toolCallRec
)

type toolCallRec struct {
	Name string
	Args json.RawMessage
}

func TestMain(m *testing.M) {
	// B06: real LLM Invoke — update connection state, then fresh-session readback.
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke_connection_state_and_readback: %v\n", reason)
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

	findConn, err := toolslocations.NewFindConnectionsTool(
		filepath.Join(root, "definitions", "find_connections.json"), locSvc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_connections: %v\n", err)
		return
	}
	updateState, err := toolslocations.NewUpdateConnectionStateTool(
		filepath.Join(root, "definitions", "update_connection_state.json"), locSvc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection_state: %v\n", err)
		return
	}
	sendMessage, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"), msgSvc, nil, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message: %v\n", err)
		return
	}

	for _, tool := range []tools.Tool{findConn, updateState, sendMessage} {
		if err := ex.Register(recordTool(tool), role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", tool.Definition().Name, err)
			return
		}
	}

	p, err := openai.NewOpenAICompatibleProvider(ex, url, model, apiKey, prompt, types.Role_Master, maxIters)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	testRepo = locGenerator(conn.DB())
	provider = p
	code = m.Run()
}

func TestInvoke_Connection_StateAndReadback(t *testing.T) {
	resetDB(t)
	resetToolSeq()
	ctx := context.Background()

	from := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Crypt Door")
	to := seedLocation(t, nil, repolocations.LocationType_ROOM, "Ossuary")

	passage := &repolocations.Connection{
		FromID:        from.ID,
		ToID:          to.ID,
		Bidirectional: false,
		State:         repositories.JSON(`{"open":false,"known_to_players":true,"lock":"iron bar"}`),
		Details:       repositories.JSON(`{"material":"stone","secret":false}`),
		Description:   "A barred special passage from the Crypt Door into the Ossuary.",
	}
	if err := testRepo.CreateConnection(ctx, passage); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	beforeDetails := normalizeJSON(passage.Details)
	beforeDesc := passage.Description
	fmt.Printf("=== Seeded ===\n  from=%d to=%d connection=%d state=%s\n",
		from.ID, to.ID, passage.ID, passage.State)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 300)) * time.Second

	// --- Invoke 1: unlock / open the passage ---
	invoke1 := uuid.NewString()
	msg1 := fmt.Sprintf(
		`Мастер узнал: ранее запертый специальный проход (Connection id=%d, from_id=%d %q → to_id=%d %q) теперь открыт.
Сначала найди связь через find_connections. Затем обнови только state через update_connection_state (full overwrite): поставь open=true, сохрани known_to_players=true и остальные нужные ключи (lock можно убрать или отметить как снятый).
Description и details не меняй другими инструментами — их нет в этом наборе.
В конце одним send_message сообщи, что проход открыт.`,
		passage.ID, from.ID, from.Name, to.ID, to.Name,
	)
	fmt.Printf("=== Invoke 1 ===\ninvoke_id: %s\nuser: %s\n", invoke1, msg1)

	invokeCtx1, cancel1 := context.WithTimeout(context.Background(), timeout)
	defer cancel1()
	if err := provider.Invoke(invokeCtx1, invoke1, providers.Request{
		Args: json.RawMessage(msg1),
	}); err != nil {
		t.Fatalf("Invoke 1: %v", err)
	}

	seq1 := takeToolSeqSince(0)
	fmt.Printf("=== Tool call sequence invoke1 (%d) ===\n", len(seq1))
	for i, rec := range seq1 {
		fmt.Printf("%d. %s args=%s\n", i+1, rec.Name, compactJSON(rec.Args))
	}
	if indexOfTool(seq1, "find_connections") < 0 {
		t.Fatal("invoke1: find_connections required")
	}
	if indexOfTool(seq1, "update_connection_state") < 0 {
		t.Fatal("invoke1: update_connection_state required")
	}
	assertJournalTools(t, invoke1, map[string]int{"send_message": 1}, []string{"find_connections", "update_connection_state"})

	got1, err := testRepo.GetConnection(ctx, passage.ID)
	if err != nil {
		t.Fatalf("GetConnection after invoke1: %v", err)
	}
	fmt.Printf("=== Connection after invoke1 ===\n%s\n", mustJSON(got1))
	var state1 map[string]any
	if err := json.Unmarshal(got1.State, &state1); err != nil {
		t.Fatalf("state unmarshal: %v", err)
	}
	if open, _ := state1["open"].(bool); !open {
		t.Fatalf("after invoke1 open must be true, state=%s", got1.State)
	}
	if known, ok := state1["known_to_players"].(bool); ok && !known {
		t.Fatalf("known_to_players should stay true when preserved, state=%s", got1.State)
	}
	if got1.Description != beforeDesc {
		t.Fatalf("description must stay unchanged, got %q", got1.Description)
	}
	if normalizeJSON(got1.Details) != beforeDetails {
		t.Fatalf("details must stay unchanged:\nbefore=%s\nafter=%s", beforeDetails, got1.Details)
	}
	if got1.FromID != from.ID || got1.ToID != to.ID || got1.Bidirectional {
		t.Fatalf("identity fields drifted: %+v", got1)
	}

	markerAfter1 := len(snapshotToolSeq())

	// --- Invoke 2: fresh session — read state from DB ---
	invoke2 := uuid.NewString()
	msg2 := fmt.Sprintf(
		`Игрок спрашивает: открыт ли сейчас специальный проход между %q (id=%d) и %q (id=%d)?
Это новый разговор без истории. Сначала заново вызови find_connections и прочитай актуальный state из результата инструмента. Затем одним send_message ответь, открыт ли проход, опираясь только на найденные данные.`,
		from.Name, from.ID, to.Name, to.ID,
	)
	fmt.Printf("=== Invoke 2 ===\ninvoke_id: %s\nuser: %s\n", invoke2, msg2)

	invokeCtx2, cancel2 := context.WithTimeout(context.Background(), timeout)
	defer cancel2()
	if err := provider.Invoke(invokeCtx2, invoke2, providers.Request{
		Args: json.RawMessage(msg2),
	}); err != nil {
		t.Fatalf("Invoke 2: %v", err)
	}

	seq2 := takeToolSeqSince(markerAfter1)
	fmt.Printf("=== Tool call sequence invoke2 (%d) ===\n", len(seq2))
	for i, rec := range seq2 {
		fmt.Printf("%d. %s args=%s\n", i+1, rec.Name, compactJSON(rec.Args))
	}
	if indexOfTool(seq2, "find_connections") < 0 {
		t.Fatal("invoke2: find_connections required (fresh read from DB)")
	}
	if indexOfTool(seq2, "update_connection_state") >= 0 {
		t.Fatal("invoke2 must not update state again")
	}
	assertJournalTools(t, invoke2, map[string]int{"send_message": 1}, []string{"find_connections", "update_connection_state"})

	got2, err := testRepo.GetConnection(ctx, passage.ID)
	if err != nil {
		t.Fatalf("GetConnection after invoke2: %v", err)
	}
	var state2 map[string]any
	if err := json.Unmarshal(got2.State, &state2); err != nil {
		t.Fatalf("state2 unmarshal: %v", err)
	}
	if open, _ := state2["open"].(bool); !open {
		t.Fatalf("DB open must remain true after invoke2, state=%s", got2.State)
	}

	text2 := lastPlayerMessage(t, invoke2)
	fmt.Printf("=== Invoke2 player message ===\n%s\n", text2)
	lower := strings.ToLower(text2)
	if !strings.Contains(lower, "открыт") &&
		!strings.Contains(lower, "open") &&
		!strings.Contains(lower, "unlocked") &&
		!strings.Contains(lower, "доступен") {
		t.Fatalf("invoke2 must report the passage is open from DB state, got %q", text2)
	}
	if strings.Contains(lower, "заперт") ||
		strings.Contains(lower, "закрыт") ||
		strings.Contains(lower, "locked") ||
		strings.Contains(lower, "closed") {
		// Allow "previously locked" phrasing only with past markers.
		if !strings.Contains(lower, "ранее") &&
			!strings.Contains(lower, "раньше") &&
			!strings.Contains(lower, "был") &&
			!strings.Contains(lower, "бывш") &&
			!strings.Contains(lower, "previous") &&
			!strings.Contains(lower, "was ") &&
			!strings.Contains(lower, "were ") {
			t.Fatalf("invoke2 must not present the passage as currently locked/closed: %q", text2)
		}
	}

	idStr := strconv.FormatUint(uint64(passage.ID), 10)
	_ = idStr
	fmt.Printf("=== Canon state ===\n%s\n", got2.State)
}

func assertJournalTools(t *testing.T, invokeID string, required map[string]int, forbiddenIdempotent []string) {
	t.Helper()
	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	fmt.Printf("=== Journal %s (%d) ===\n", invokeID, len(journaled))
	counts := map[string]int{}
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
		for _, name := range forbiddenIdempotent {
			if req.Tool == name {
				t.Fatalf("%s is idempotent and must not be journaled", name)
			}
		}
		counts[req.Tool]++
		if req.Status != journal.Status_Completed {
			t.Fatalf("%s status: want completed, got %s", req.Tool, req.Status)
		}
	}
	for tool, want := range required {
		if counts[tool] < want {
			t.Fatalf("want at least %d %s journal rows, got %d", want, tool, counts[tool])
		}
	}
}

func lastPlayerMessage(t *testing.T, invokeID string) string {
	t.Helper()
	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected send_message text")
	}
	return messages[len(messages)-1].Text
}

func seedLocation(t *testing.T, parentID *repositories.ID, locType repolocations.LocationType, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    parentID,
		Type:        locType,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed %q: %v", name, err)
	}
	return loc
}

func normalizeJSON(raw repositories.JSON) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

type recordingTool struct {
	inner tools.Tool
}

func recordTool(inner tools.Tool) tools.Tool {
	return &recordingTool{inner: inner}
}

func (r *recordingTool) Definition() definition.Definition { return r.inner.Definition() }
func (r *recordingTool) IsIdempotent() bool                { return r.inner.IsIdempotent() }

func (r *recordingTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	toolSeqMu.Lock()
	toolSeq = append(toolSeq, toolCallRec{
		Name: r.inner.Definition().Name,
		Args: append(json.RawMessage(nil), args...),
	})
	toolSeqMu.Unlock()
	return r.inner.Execute(ctx, args)
}

func resetToolSeq() {
	toolSeqMu.Lock()
	toolSeq = nil
	toolSeqMu.Unlock()
}

func snapshotToolSeq() []toolCallRec {
	toolSeqMu.Lock()
	defer toolSeqMu.Unlock()
	out := make([]toolCallRec, len(toolSeq))
	copy(out, toolSeq)
	return out
}

func takeToolSeqSince(start int) []toolCallRec {
	all := snapshotToolSeq()
	if start >= len(all) {
		return nil
	}
	return all[start:]
}

func indexOfTool(seq []toolCallRec, name string) int {
	for i, rec := range seq {
		if rec.Name == name {
			return i
		}
	}
	return -1
}

func compactJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func mustJSON(v any) string {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
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
