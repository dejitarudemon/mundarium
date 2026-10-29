package connection_full_replacement_test

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

var partialUpdateTools = []string{
	"update_connection_state",
	"update_connection_details",
	"update_connection_description",
	"update_connection_to",
	"update_connection_bidirectional",
	"update_connection_expires_at",
}

func TestMain(m *testing.M) {
	// B07: real LLM Invoke — full update_connection replacement (not partial patches).
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke_update_connection_full_replacement: %v\n", reason)
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
	defs := filepath.Join(root, "definitions")

	makers := []struct {
		file string
		make func(string) (tools.Tool, error)
	}{
		{"find_connections.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewFindConnectionsTool(p, locSvc, log)
		}},
		{"update_connection.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionTool(p, locSvc, log)
		}},
		{"update_connection_state.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionStateTool(p, locSvc, log)
		}},
		{"update_connection_details.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDetailsTool(p, locSvc, log)
		}},
		{"update_connection_description.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionDescriptionTool(p, locSvc, log)
		}},
		{"update_connection_to.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionToTool(p, locSvc, log)
		}},
		{"update_connection_bidirectional.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionBidirectionalTool(p, locSvc, log)
		}},
		{"update_connection_expires_at.json", func(p string) (tools.Tool, error) {
			return toolslocations.NewUpdateConnectionExpiresAtTool(p, locSvc, log)
		}},
	}

	for _, item := range makers {
		tool, err := item.make(filepath.Join(defs, item.file))
		if err != nil {
			fmt.Fprintf(os.Stderr, "new %s: %v\n", item.file, err)
			return
		}
		if err := ex.Register(recordTool(tool), role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", tool.Definition().Name, err)
			return
		}
	}

	sendMessage, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(defs, "send_message.json"), msgSvc, nil, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message: %v\n", err)
		return
	}
	if err := ex.Register(recordTool(sendMessage), role); err != nil {
		fmt.Fprintf(os.Stderr, "register send_message: %v\n", err)
		return
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

func TestInvoke_UpdateConnection_FullReplacement(t *testing.T) {
	resetDB(t)
	resetToolSeq()
	ctx := context.Background()

	from := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Gatehouse")
	oldTo := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Old Courtyard")
	newTo := seedLocation(t, nil, repolocations.LocationType_STRUCTURE, "Ruined Chapel")

	expires := int64(1_700_000_000)
	seed := &repolocations.Connection{
		FromID:        from.ID,
		ToID:          oldTo.ID,
		Bidirectional: true,
		State:         repositories.JSON(`{"open":true,"known_to_players":true,"guard":"alive"}`),
		Details:       repositories.JSON(`{"material":"stone","secret":"sigil"}`),
		Description:   "A sealed bidirectional portal under the gatehouse.",
		ExpiresAt:     &expires,
	}
	if err := testRepo.CreateConnection(ctx, seed); err != nil {
		t.Fatalf("seed connection: %v", err)
	}
	fmt.Printf("=== Seeded ===\n  from=%d oldTo=%d newTo=%d conn=%d\n%s\n",
		from.ID, oldTo.ID, newTo.ID, seed.ID, mustJSON(seed))

	const newDesc = "The portal has collapsed into a one-way rubble chute."

	userMessage := fmt.Sprintf(
		`Нужно полностью перезаписать Connection id=%d через инструмент update_connection (не через update_connection_* патчи).

Сначала найди связь find_connections. Затем вызови update_connection со ВСЕМИ обязательными полями:
- id=%d
- to_id=%d  (новый endpoint; from_id=%d не меняется и в аргументах не передаётся)
- bidirectional=false
- state={}
- details={}
- description=%q
- expires_at=null

Частичные update_connection_state / details / description / to / bidirectional / expires_at НЕ используй.
В конце одним send_message сообщи id обновлённой связи.`,
		seed.ID, seed.ID, newTo.ID, from.ID, newDesc,
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
		t.Fatal("find_connections must be called first")
	}
	if indexOfTool(seq, "update_connection") < 0 {
		t.Fatal("update_connection (full) must be called")
	}
	for _, name := range partialUpdateTools {
		if indexOfTool(seq, name) >= 0 {
			t.Fatalf("must not use partial tool %s for full replacement", name)
		}
	}
	if indexOfTool(seq, "send_message") < 0 {
		t.Fatal("send_message required")
	}

	// Full update args must include false / {} / null.
	fullArgs := firstToolArgs(seq, "update_connection")
	assertFullUpdateArgs(t, fullArgs, seed.ID, newTo.ID, newDesc)

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
		if req.Tool == "find_connections" || req.Tool == "update_connection" || strings.HasPrefix(req.Tool, "update_connection_") {
			t.Fatalf("idempotent tool %s must not be journaled", req.Tool)
		}
		if req.Tool == "send_message" && req.Status != journal.Status_Completed {
			t.Fatalf("send_message status: want completed, got %s", req.Status)
		}
	}

	got, err := testRepo.GetConnection(ctx, seed.ID)
	if err != nil {
		t.Fatalf("GetConnection: %v", err)
	}
	fmt.Printf("=== Connection after full replacement ===\n%s\n", mustJSON(got))

	if got.FromID != from.ID {
		t.Fatalf("from_id must stay %d, got %d", from.ID, got.FromID)
	}
	if got.ToID != newTo.ID {
		t.Fatalf("to_id: want %d, got %d", newTo.ID, got.ToID)
	}
	if got.Bidirectional {
		t.Fatal("bidirectional must be false")
	}
	if got.Description != newDesc {
		t.Fatalf("description: want %q, got %q", newDesc, got.Description)
	}
	if got.ExpiresAt != nil {
		t.Fatalf("expires_at must be NULL, got %#v", got.ExpiresAt)
	}
	assertEmptyObject(t, "state", got.State)
	assertEmptyObject(t, "details", got.Details)
	assertNoKey(t, "state", got.State, "open", "known_to_players", "guard")
	assertNoKey(t, "details", got.Details, "material", "secret")

	var connCount int64
	if err := testConn.DB().Model(&repolocations.Connection{}).Count(&connCount).Error; err != nil {
		t.Fatalf("count connections: %v", err)
	}
	if connCount != 1 {
		t.Fatalf("want exactly 1 connection, got %d", connCount)
	}

	text := lastPlayerMessage(t, invokeID)
	fmt.Printf("=== Player message ===\n%s\n", text)
	idStr := strconv.FormatUint(uint64(seed.ID), 10)
	if !strings.Contains(text, idStr) {
		t.Fatalf("message must mention connection id=%s, got %q", idStr, text)
	}
}

func assertFullUpdateArgs(t *testing.T, args json.RawMessage, id, toID repositories.ID, desc string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(args, &m); err != nil {
		t.Fatalf("unmarshal update_connection args: %v", err)
	}
	if gotID, ok := asID(m["id"]); !ok || gotID != id {
		t.Fatalf("update_connection id: want %d, got %#v", id, m["id"])
	}
	if gotTo, ok := asID(m["to_id"]); !ok || gotTo != toID {
		t.Fatalf("update_connection to_id: want %d, got %#v", toID, m["to_id"])
	}
	if bi, ok := m["bidirectional"].(bool); !ok || bi {
		t.Fatalf("update_connection bidirectional must be false, got %#v", m["bidirectional"])
	}
	if _, has := m["expires_at"]; !has {
		t.Fatal("update_connection must include expires_at")
	}
	if m["expires_at"] != nil {
		t.Fatalf("expires_at must be null in args, got %#v", m["expires_at"])
	}
	assertArgEmptyObject(t, "state", m["state"])
	assertArgEmptyObject(t, "details", m["details"])
	if d, _ := m["description"].(string); d != desc {
		t.Fatalf("description arg: want %q, got %#v", desc, m["description"])
	}
	if _, hasFrom := m["from_id"]; hasFrom {
		t.Fatal("update_connection must not pass from_id")
	}
}

func assertArgEmptyObject(t *testing.T, label string, v any) {
	t.Helper()
	obj, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("%s arg must be object, got %#v", label, v)
	}
	if len(obj) != 0 {
		t.Fatalf("%s arg must be empty {}, got %#v", label, v)
	}
}

func assertEmptyObject(t *testing.T, label string, raw repositories.JSON) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s: %v (%s)", label, err, raw)
	}
	if len(obj) != 0 {
		t.Fatalf("%s: want {}, got %s", label, raw)
	}
}

func assertNoKey(t *testing.T, label string, raw repositories.JSON, keys ...string) {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatalf("%s: %v", label, err)
	}
	for _, key := range keys {
		if _, ok := obj[key]; ok {
			t.Fatalf("%s still has old key %q: %s", label, key, raw)
		}
	}
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

func firstToolArgs(seq []toolCallRec, name string) json.RawMessage {
	for _, rec := range seq {
		if rec.Name == name {
			return rec.Args
		}
	}
	return nil
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
