package connection_bidirectional_reverse_test

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
	// B03: real LLM Invoke — find bidirectional reverse without inventing B→A.
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke_find_connection_bidirectional_reverse: %v\n", reason)
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
	createConn, err := toolslocations.NewCreateConnectionTool(
		filepath.Join(root, "definitions", "create_connection.json"), locSvc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_connection: %v\n", err)
		return
	}
	sendMessage, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"), msgSvc, nil, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message: %v\n", err)
		return
	}

	for _, tool := range []tools.Tool{findConn, createConn, sendMessage} {
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

func TestInvoke_FindConnection_BidirectionalReverse(t *testing.T) {
	resetDB(t)
	resetToolSeq()
	ctx := context.Background()

	locA := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Obsidian Gate")
	locB := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Moonlit Plaza")

	portal := &repolocations.Connection{
		FromID:        locA.ID,
		ToID:          locB.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":true,"known_to_players":true}`),
		Details:       repositories.JSON(`{"kind":"portal","rune":"silver"}`),
		Description:   "A bidirectional silver portal between the Obsidian Gate and the Moonlit Plaza.",
	}
	if err := testRepo.CreateConnection(ctx, portal); err != nil {
		t.Fatalf("seed portal: %v", err)
	}
	before := snapshotConnection(t, portal.ID)
	fmt.Printf("=== Seeded ===\n  A=%d (%s) B=%d (%s) portal=%d bidirectional=true\n%s\n",
		locA.ID, locA.Name, locB.ID, locB.Name, portal.ID, mustJSON(before))

	userMessage := fmt.Sprintf(
		`Игрок сейчас находится в локации B id=%d (%q) и спрашивает: есть ли установленный специальный переход обратно в A id=%d (%q)?

Проверь канон через find_connections. Для обратного направления двусторонней связи используй include_bidirectional_reverse=true с from_id=%d to_id=%d, либо location_id=%d.
Не создавай вторую запись B→A, если уже есть bidirectional A→B.
В конце одним send_message ответь игроку: укажи id найденной связи и что переход двусторонний.`,
		locB.ID, locB.Name, locA.ID, locA.Name,
		locB.ID, locA.ID, locB.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 300)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(invokeCtx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	seq := snapshotToolSeq()
	fmt.Printf("=== Tool call sequence (%d) ===\n", len(seq))
	for i, rec := range seq {
		fmt.Printf("%d. %s args=%s\n", i+1, rec.Name, compactJSON(rec.Args))
	}

	if indexOfTool(seq, "find_connections") < 0 {
		t.Fatal("find_connections must be called")
	}
	if indexOfTool(seq, "create_connection") >= 0 {
		t.Fatal("create_connection must not invent a reverse B→A row")
	}
	if !findUsedDirectedSearch(seq, locB.ID, locA.ID) {
		t.Fatal("find_connections must use directed reverse search (include_bidirectional_reverse / either_direction / location_id)")
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	var createRows, sendRows int
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
		switch req.Tool {
		case "find_connections":
			t.Fatal("find_connections must not be journaled")
		case "create_connection":
			createRows++
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message status: want completed, got %s", req.Status)
			}
		}
	}
	if createRows != 0 {
		t.Fatalf("create_connection journal rows: want 0, got %d", createRows)
	}
	if sendRows == 0 {
		t.Fatal("expected send_message")
	}

	var connections []repolocations.Connection
	if err := testConn.DB().Order("id asc").Find(&connections).Error; err != nil {
		t.Fatalf("load connections: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("want exactly 1 connection (no B→A invent), got %d", len(connections))
	}
	got := connections[0]
	fmt.Printf("=== Final connection ===\n%s\n", mustJSON(got))
	if got.ID != portal.ID || got.FromID != locA.ID || got.ToID != locB.ID || !got.Bidirectional {
		t.Fatalf("portal must stay A→B bidirectional id=%d, got id=%d %d→%d bi=%v",
			portal.ID, got.ID, got.FromID, got.ToID, got.Bidirectional)
	}
	if snapshotConnection(t, portal.ID) != before {
		t.Fatal("seeded portal fields mutated")
	}

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
	fmt.Printf("=== Player message ===\n%s\n", text)
	idStr := strconv.FormatUint(uint64(portal.ID), 10)
	if !strings.Contains(text, idStr) {
		t.Fatalf("message must include connection id=%s, got %q", idStr, text)
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "bidirectional") &&
		!strings.Contains(lower, "двусторон") &&
		!strings.Contains(lower, "в обе сторон") &&
		!strings.Contains(lower, "обратн") {
		t.Fatalf("message should acknowledge bidirectional travel, got %q", text)
	}
}

func findUsedDirectedSearch(seq []toolCallRec, fromB, toA repositories.ID) bool {
	for _, rec := range seq {
		if rec.Name != "find_connections" {
			continue
		}
		var args map[string]any
		if err := json.Unmarshal(rec.Args, &args); err != nil {
			continue
		}
		if locID, ok := asID(args["location_id"]); ok && (locID == fromB || locID == toA) {
			return true
		}
		fromID, hasFrom := asID(args["from_id"])
		toID, hasTo := asID(args["to_id"])
		if hasFrom && hasTo && fromID == fromB && toID == toA {
			if truthy(args["include_bidirectional_reverse"]) || truthy(args["either_direction"]) {
				return true
			}
		}
		// location_id-only or from_id=B without to is also acceptable directed discovery
		if hasFrom && fromID == fromB {
			return true
		}
	}
	return false
}

func asID(v any) (repositories.ID, bool) {
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0, false
		}
		return repositories.ID(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil || i <= 0 {
			return 0, false
		}
		return repositories.ID(i), true
	default:
		return 0, false
	}
}

func truthy(v any) bool {
	b, ok := v.(bool)
	return ok && b
}

type connSnap struct {
	FromID        repositories.ID
	ToID          repositories.ID
	Bidirectional bool
	State         string
	Details       string
	Description   string
	ExpiresNull   bool
}

func snapshotConnection(t *testing.T, id repositories.ID) connSnap {
	t.Helper()
	got, err := testRepo.GetConnection(context.Background(), id)
	if err != nil {
		t.Fatalf("GetConnection(%d): %v", id, err)
	}
	return connSnap{
		FromID:        got.FromID,
		ToID:          got.ToID,
		Bidirectional: got.Bidirectional,
		State:         normalizeJSON(got.State),
		Details:       normalizeJSON(got.Details),
		Description:   got.Description,
		ExpiresNull:   got.ExpiresAt == nil,
	}
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
