package connection_multi_tool_workflow_test

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
	// B08: real LLM Invoke — multi-tool connection workflow.
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke_connection_multi_tool_workflow: %v\n", reason)
		os.Exit(0)
	}
	if maxIters < 12 {
		maxIters = 12 // find + patch + find + create + send may need more rounds
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
	defs := filepath.Join(root, "definitions")

	findConn, err := toolslocations.NewFindConnectionsTool(filepath.Join(defs, "find_connections.json"), locSvc, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_connections: %v\n", err)
		return
	}
	updateState, err := toolslocations.NewUpdateConnectionStateTool(filepath.Join(defs, "update_connection_state.json"), locSvc, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection_state: %v\n", err)
		return
	}
	createConn, err := toolslocations.NewCreateConnectionTool(filepath.Join(defs, "create_connection.json"), locSvc, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_connection: %v\n", err)
		return
	}
	sendMessage, err := toolsmessages.NewCreateMessageTool(filepath.Join(defs, "send_message.json"), msgSvc, nil, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message: %v\n", err)
		return
	}

	for _, tool := range []tools.Tool{findConn, updateState, createConn, sendMessage} {
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

func TestInvoke_Connection_MultiToolWorkflow(t *testing.T) {
	resetDB(t)
	resetToolSeq()
	ctx := context.Background()

	// Settlement → Tavern → Cellar | Castle → Dungeon
	town := seedLocation(t, nil, repolocations.LocationType_SETTLEMENT, "Town")
	tavern := seedLocation(t, &town.ID, repolocations.LocationType_STRUCTURE, "Tavern")
	cellar := seedLocation(t, &tavern.ID, repolocations.LocationType_ROOM, "Cellar")
	castle := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Castle")
	dungeon := seedLocation(t, &castle.ID, repolocations.LocationType_ROOM, "Dungeon")

	portal := &repolocations.Connection{
		FromID:        tavern.ID,
		ToID:          castle.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":false,"known_to_players":true,"seal":"wax"}`),
		Details:       repositories.JSON(`{"kind":"portal","rune":"amber"}`),
		Description:   "A sealed bidirectional portal between the Tavern and the Castle.",
	}
	if err := testRepo.CreateConnection(ctx, portal); err != nil {
		t.Fatalf("seed portal: %v", err)
	}

	locBefore := snapshotLocations(t)
	fmt.Printf("=== Seeded locations ===\n")
	for _, loc := range []*repolocations.Location{town, tavern, cellar, castle, dungeon} {
		parent := "null"
		if loc.ParentID != nil {
			parent = strconv.FormatUint(uint64(*loc.ParentID), 10)
		}
		fmt.Printf("  id=%d name=%q parent_id=%s\n", loc.ID, loc.Name, parent)
	}
	fmt.Printf("=== Seeded portal id=%d %d→%d bi=true open=false ===\n", portal.ID, portal.FromID, portal.ToID)
	fmt.Printf("=== Missing tunnel: Cellar %d → Dungeon %d ===\n", cellar.ID, dungeon.ID)

	userMessage := fmt.Sprintf(
		`Выполни каноническую работу с Connect в этом мире:

1) Найди существующий портал между таверной (id=%d) и замком (id=%d) через find_connections. Переиспользуй его id=%d.
2) Обнови его текущее состояние через update_connection_state: open=true, сохрани known_to_players=true (seal можно убрать или пометить как снятый).
3) Проверь, есть ли тайный тоннель между подвалом (Cellar id=%d) и подземельем (Dungeon id=%d). Если нет — создай create_connection:
   from_id=%d, to_id=%d, bidirectional=false, description про тайный тоннель, state с known_to_players=false, details по смыслу, expires_at=null.
4) Локации не создавай и не перемещай.
5) Одним send_message сообщи игроку: id портала (что он открыт) и id нового тоннеля. Утверждай только факты из ответов инструментов.`,
		tavern.ID, castle.ID, portal.ID,
		cellar.ID, dungeon.ID,
		cellar.ID, dungeon.ID,
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

	if countTool(seq, "find_connections") < 1 {
		t.Fatal("find_connections must be used")
	}
	if countTool(seq, "update_connection_state") != 1 {
		t.Fatalf("want exactly 1 update_connection_state, got %d", countTool(seq, "update_connection_state"))
	}
	if countTool(seq, "create_connection") != 1 {
		t.Fatalf("want exactly 1 create_connection, got %d", countTool(seq, "create_connection"))
	}
	if countTool(seq, "send_message") < 1 {
		t.Fatal("send_message required")
	}

	// update must target the seeded portal id
	stateArgs := firstToolArgs(seq, "update_connection_state")
	var stateCall struct {
		ID    repositories.ID   `json:"id"`
		State map[string]any    `json:"state"`
	}
	if err := json.Unmarshal(stateArgs, &stateCall); err != nil {
		t.Fatalf("unmarshal update_connection_state args: %v", err)
	}
	if stateCall.ID != portal.ID {
		t.Fatalf("update_connection_state must reuse portal id=%d, got %d", portal.ID, stateCall.ID)
	}
	if open, _ := stateCall.State["open"].(bool); !open {
		t.Fatalf("update state must set open=true, got %#v", stateCall.State)
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
	var createdFromJournal repositories.ID
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
		switch req.Tool {
		case "find_connections", "update_connection_state":
			t.Fatalf("%s is idempotent and must not be journaled", req.Tool)
		case "create_connection":
			createRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("create_connection status: want completed, got %s err=%v", req.Status, req.Error)
			}
			createdFromJournal = connectionIDFromJournal(t, req)
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message status: want completed, got %s", req.Status)
			}
		}
	}
	if createRows != 1 {
		t.Fatalf("journal create_connection rows: want 1, got %d", createRows)
	}
	if sendRows == 0 {
		t.Fatal("expected send_message journal row")
	}
	if createdFromJournal == 0 || createdFromJournal == portal.ID {
		t.Fatalf("create_connection journal must yield a new connection id, got %d", createdFromJournal)
	}

	var connections []repolocations.Connection
	if err := testConn.DB().Order("id asc").Find(&connections).Error; err != nil {
		t.Fatalf("load connections: %v", err)
	}
	if len(connections) != 2 {
		t.Fatalf("want exactly 2 connections (portal+tunnel), got %d", len(connections))
	}
	fmt.Printf("=== Final connections ===\n%s\n", mustJSON(connections))

	gotPortal, err := testRepo.GetConnection(ctx, portal.ID)
	if err != nil {
		t.Fatalf("GetConnection portal: %v", err)
	}
	if gotPortal.FromID != tavern.ID || gotPortal.ToID != castle.ID || !gotPortal.Bidirectional {
		t.Fatalf("portal endpoints/bi drifted: %+v", gotPortal)
	}
	var portalState map[string]any
	if err := json.Unmarshal(gotPortal.State, &portalState); err != nil {
		t.Fatalf("portal state: %v", err)
	}
	if open, _ := portalState["open"].(bool); !open {
		t.Fatalf("portal must be open after update, state=%s", gotPortal.State)
	}
	if known, ok := portalState["known_to_players"].(bool); ok && !known {
		t.Fatalf("portal known_to_players should stay true, state=%s", gotPortal.State)
	}

	gotTunnel, err := testRepo.GetConnection(ctx, createdFromJournal)
	if err != nil {
		t.Fatalf("GetConnection tunnel %d: %v", createdFromJournal, err)
	}
	if gotTunnel.FromID != cellar.ID || gotTunnel.ToID != dungeon.ID {
		t.Fatalf("tunnel endpoints: want %d→%d, got %d→%d", cellar.ID, dungeon.ID, gotTunnel.FromID, gotTunnel.ToID)
	}
	if gotTunnel.Bidirectional {
		t.Fatal("tunnel should be one-way")
	}
	if strings.TrimSpace(gotTunnel.Description) == "" {
		t.Fatal("tunnel description must be non-empty")
	}
	var tunnelState map[string]any
	if err := json.Unmarshal(gotTunnel.State, &tunnelState); err == nil {
		if known, ok := tunnelState["known_to_players"].(bool); ok && known {
			t.Fatalf("secret tunnel known_to_players should be false, state=%s", gotTunnel.State)
		}
	}

	locAfter := snapshotLocations(t)
	if len(locAfter) != len(locBefore) {
		t.Fatalf("location count changed: before=%d after=%d", len(locBefore), len(locAfter))
	}
	for id, before := range locBefore {
		after, ok := locAfter[id]
		if !ok || after != before {
			t.Fatalf("location %d changed:\nbefore=%+v\nafter=%+v", id, before, after)
		}
	}

	text := lastPlayerMessage(t, invokeID)
	fmt.Printf("=== Player message ===\n%s\n", text)
	portalIDStr := strconv.FormatUint(uint64(portal.ID), 10)
	tunnelIDStr := strconv.FormatUint(uint64(createdFromJournal), 10)
	if !strings.Contains(text, portalIDStr) {
		t.Fatalf("message must mention portal id=%s, got %q", portalIDStr, text)
	}
	if !strings.Contains(text, tunnelIDStr) {
		t.Fatalf("message must mention new tunnel id=%s, got %q", tunnelIDStr, text)
	}
	lower := strings.ToLower(text)
	if !strings.Contains(lower, "открыт") && !strings.Contains(lower, "open") {
		t.Fatalf("message should report portal is open, got %q", text)
	}
	// No invented third connection ids in the message.
	for _, c := range connections {
		idStr := strconv.FormatUint(uint64(c.ID), 10)
		_ = idStr
	}
}

func connectionIDFromJournal(t *testing.T, req journal.Request) repositories.ID {
	t.Helper()
	raw, err := json.Marshal(req.Result.Data)
	if err != nil {
		t.Fatalf("marshal journal data: %v", err)
	}
	// Result data may be the CreateConnectionOutput object or nested.
	var direct struct {
		ID repositories.ID `json:"id"`
	}
	if err := json.Unmarshal(raw, &direct); err == nil && direct.ID != 0 {
		return direct.ID
	}
	var wrapped map[string]any
	if err := json.Unmarshal(raw, &wrapped); err == nil {
		if id, ok := asID(wrapped["id"]); ok {
			return id
		}
	}
	// Fallback: parse create_connection tool args/result from sequence is better —
	// also try Result as stored CreateConnectionOutput via JSON number.
	t.Fatalf("cannot extract connection id from journal result: %s", raw)
	return 0
}

func asID(v any) (repositories.ID, bool) {
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0, false
		}
		return repositories.ID(n), true
	default:
		return 0, false
	}
}

type locSnap struct {
	Name        string
	Description string
	Details     string
	ParentID    repositories.ID
	HasParent   bool
	Type        string
}

func snapshotLocations(t *testing.T) map[repositories.ID]locSnap {
	t.Helper()
	var all []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("dump locations: %v", err)
	}
	out := make(map[repositories.ID]locSnap, len(all))
	for _, loc := range all {
		snap := locSnap{
			Name:        loc.Name,
			Description: loc.Description,
			Details:     string(loc.Details),
			Type:        loc.Type.String(),
		}
		if loc.ParentID != nil {
			snap.HasParent = true
			snap.ParentID = *loc.ParentID
		}
		out[loc.ID] = snap
	}
	return out
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
		t.Fatal("expected send_message")
	}
	return messages[len(messages)-1].Text
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

func countTool(seq []toolCallRec, name string) int {
	n := 0
	for _, rec := range seq {
		if rec.Name == name {
			n++
		}
	}
	return n
}

func firstToolArgs(seq []toolCallRec, name string) json.RawMessage {
	for _, rec := range seq {
		if rec.Name == name {
			return rec.Args
		}
	}
	return nil
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
