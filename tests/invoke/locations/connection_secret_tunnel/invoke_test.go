package connection_secret_tunnel_test

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
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
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
	// B01: real LLM Invoke create_connection secret tunnel.
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke_create_connection_secret_tunnel: %v\n", reason)
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

func TestInvoke_CreateConnection_SecretTunnel(t *testing.T) {
	resetDB(t)
	resetToolSeq()
	ctx := context.Background()

	// Town → Tavern → Cellar | Castle → Dungeon
	town := seedLocation(t, nil, repolocations.LocationType_SETTLEMENT, "Town")
	tavern := seedLocation(t, &town.ID, repolocations.LocationType_STRUCTURE, "Tavern")
	cellar := seedLocation(t, &tavern.ID, repolocations.LocationType_ROOM, "Cellar")
	castle := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Castle")
	dungeon := seedLocation(t, &castle.ID, repolocations.LocationType_ROOM, "Dungeon")

	locBefore := snapshotLocations(t)
	fmt.Printf("=== Seeded locations ===\n")
	for _, loc := range []*repolocations.Location{town, tavern, cellar, castle, dungeon} {
		parent := "null"
		if loc.ParentID != nil {
			parent = strconv.FormatUint(uint64(*loc.ParentID), 10)
		}
		fmt.Printf("  id=%d name=%q parent_id=%s type=%s\n", loc.ID, loc.Name, parent, loc.Type.String())
	}

	userMessage := fmt.Sprintf(
		`Между подвалом таверны (location id=%d, name=Cellar) и подземельем замка (location id=%d, name=Dungeon) нужно зафиксировать тайный тоннель как канонический Connect.

Сначала проверь существующие связи через find_connections (например location_id=%d и/или from_id/to_id). Если явной связи ещё нет — создай её через create_connection:
- from_id=%d, to_id=%d
- bidirectional=false
- description про тайный тоннель
- state с known_to_players=false (и по желанию open=false)
- details с краткими физическими характеристиками
- expires_at=null

Локации не создавай и не перемещай. В конце одним send_message сообщи игроку id созданной связи и что тоннель зафиксирован.`,
		cellar.ID, dungeon.ID, cellar.ID, cellar.ID, dungeon.ID,
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

	findIdx := indexOfTool(seq, "find_connections")
	createIdx := indexOfTool(seq, "create_connection")
	if findIdx < 0 {
		t.Fatal("find_connections must be called before creating")
	}
	if createIdx < 0 {
		t.Fatal("create_connection must be called")
	}
	if findIdx > createIdx {
		t.Fatalf("find_connections must run before create_connection: find=%d create=%d", findIdx, createIdx)
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
			t.Fatalf("find_connections is idempotent and must not be journaled, got %#v", req)
		case "create_connection":
			createRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("create_connection status: want completed, got %s err=%v", req.Status, req.Error)
			}
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message status: want completed, got %s", req.Status)
			}
		}
	}
	if createRows != 1 {
		t.Fatalf("want exactly 1 create_connection journal row, got %d", createRows)
	}
	if sendRows == 0 {
		t.Fatal("expected send_message journal row")
	}

	var connections []repolocations.Connection
	if err := testConn.DB().Order("id asc").Find(&connections).Error; err != nil {
		t.Fatalf("load connections: %v", err)
	}
	if len(connections) != 1 {
		t.Fatalf("want exactly 1 connection, got %d", len(connections))
	}
	conn := connections[0]
	fmt.Printf("=== Final connection ===\n%s\n", mustJSON(conn))

	if conn.FromID != cellar.ID || conn.ToID != dungeon.ID {
		t.Fatalf("endpoints: want %d→%d, got %d→%d", cellar.ID, dungeon.ID, conn.FromID, conn.ToID)
	}
	if conn.Bidirectional {
		t.Fatal("want bidirectional=false for one-way secret tunnel")
	}
	if strings.TrimSpace(conn.Description) == "" {
		t.Fatal("description must be non-empty")
	}
	descLower := strings.ToLower(conn.Description)
	if !strings.Contains(descLower, "tunnel") &&
		!strings.Contains(descLower, "тоннел") &&
		!strings.Contains(descLower, "secret") &&
		!strings.Contains(descLower, "тайный") &&
		!strings.Contains(descLower, "тайного") &&
		!strings.Contains(descLower, "passage") &&
		!strings.Contains(descLower, "проход") {
		t.Fatalf("description should reflect a secret tunnel scenario, got %q", conn.Description)
	}
	assertJSONObject(t, "state", conn.State)
	assertJSONObject(t, "details", conn.Details)
	if conn.ExpiresAt != nil {
		t.Fatalf("expires_at must be null, got %#v", conn.ExpiresAt)
	}

	var stateObj map[string]any
	if err := json.Unmarshal(conn.State, &stateObj); err != nil {
		t.Fatalf("state unmarshal: %v", err)
	}
	if v, ok := stateObj["known_to_players"]; ok {
		if known, ok := v.(bool); ok && known {
			t.Fatal("known_to_players should be false for a secret tunnel")
		}
	}

	locAfter := snapshotLocations(t)
	if len(locAfter) != len(locBefore) {
		t.Fatalf("location count changed: before=%d after=%d", len(locBefore), len(locAfter))
	}
	for id, before := range locBefore {
		after, ok := locAfter[id]
		if !ok {
			t.Fatalf("location %d disappeared", id)
		}
		if after != before {
			t.Fatalf("location %d mutated:\nbefore=%+v\nafter=%+v", id, before, after)
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
		t.Fatal("expected player-facing send_message")
	}
	text := messages[len(messages)-1].Text
	fmt.Printf("=== Player message ===\n%s\n", text)
	idStr := strconv.FormatUint(uint64(conn.ID), 10)
	if !strings.Contains(text, idStr) {
		t.Fatalf("player message should mention connection id=%s, got %q", idStr, text)
	}

	_ = ctx
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

func assertJSONObject(t *testing.T, label string, raw repositories.JSON) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s must be a JSON object: %v (%s)", label, err, raw)
	}
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
