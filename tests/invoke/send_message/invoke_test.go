package send_message_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/providers"
	openai "github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const userMessage = "Привет! Доступные тулзы: send_message."

var (
	testConn  *pgstorage.Connection
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip send_message Invoke tests: %v\n", reason)
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

	generator := func(db *gorm.DB) *repomessages.MessagesRepository {
		repo, err := repomessages.NewMessagesRepository(db, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := svcmessages.NewService(generator, conn, log)
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

	tool, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(testutil.RepoRoot(), "definitions", "send_message.json"),
		svc,
		nil,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message tool: %v\n", err)
		return
	}
	if err := ex.Register(tool, []types.Role{types.Role_Master}); err != nil {
		fmt.Fprintf(os.Stderr, "register tool: %v\n", err)
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
	provider = p
	code = m.Run()
}

func TestInvoke_SendMessage(t *testing.T) {
	resetDB(t)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, "send_message").
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal requests: %v", err)
	}
	if len(journaled) == 0 {
		t.Fatal("expected at least one send_message row in execution_journal_requests")
	}
	for _, req := range journaled {
		if req.Status != journal.Status_Completed {
			t.Fatalf("journal request %s: want status %s, got %s", req.ID, journal.Status_Completed, req.Status)
		}
	}

	var saved []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&saved).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(saved) == 0 {
		t.Fatal("expected at least one message from send_message tool call")
	}

	fmt.Printf("=== Journaled tool calls (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("id=%s tool=%s status=%s\n", req.ID, req.Tool, req.Status)
	}
	fmt.Printf("=== Saved messages (%d) ===\n", len(saved))
	for _, m := range saved {
		fmt.Printf("seq=%d id=%d from=%s text=%q\n", m.SeqNum, m.ID, m.From, m.Text)
	}
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, execution_journal_requests
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
