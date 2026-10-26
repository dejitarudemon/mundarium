package tests

import (
	"bufio"
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
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

const (
	envURL        = "TEST_LLM_STUDIO_URL"
	envModel      = "TEST_LLM_STUDIO_MODEL"
	envAPIKey     = "TEST_LLM_STUDIO_API_KEY"
	envMaxIters   = "TEST_LLM_STUDIO_MAX_ITERS"
	envTimeoutSec = "TEST_LLM_STUDIO_TIMEOUT_SEC"

	userMessage = "Отправь одно приветственное сообщение через send_message. После успешного выполнения инструмента задача завершена. Не вызывай send_message повторно. Заверши работу обычным ответом без вызовов инструментов."
)

var (
	testConn *pgstorage.Connection
	provider *openai.OpenAICompatibleProvider
)

type systemPromptFile struct {
	Prompt string `json:"prompt"`
}

func TestMain(m *testing.M) {
	root := repoRoot()
	here := testDir()

	envPath := filepath.Join(root, ".env")
	if err := loadDotEnv(envPath); err != nil {
		fmt.Fprintf(os.Stderr, "skip llm studio Invoke tests: %v\n", err)
		os.Exit(0)
	}

	url := strings.TrimSpace(os.Getenv(envURL))
	model := strings.TrimSpace(os.Getenv(envModel))
	if url == "" || model == "" {
		fmt.Fprintf(os.Stderr, "skip llm studio Invoke tests: %s and %s are required in .env\n", envURL, envModel)
		os.Exit(0)
	}

	apiKey := os.Getenv(envAPIKey)
	maxIters := envInt(envMaxIters, 8)

	prompt, err := loadSystemPrompt(filepath.Join(here, "invoke_system_prompt.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load system prompt: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("mundarium"),
		postgres.WithUsername("mundarium"),
		postgres.WithPassword("mundarium"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}

	code := 1
	defer func() {
		_ = container.Terminate(ctx)
		os.Exit(code)
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres dsn: %v\n", err)
		return
	}

	conn, err := pgstorage.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open postgres: %v\n", err)
		return
	}

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
		filepath.Join(root, "definitions", "send_message.json"),
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
		ex,
		url,
		model,
		apiKey,
		prompt,
		types.Role_Master,
		maxIters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	provider = p
	code = m.Run()
}

func TestInvoke_LLMStudio_SendMessage(t *testing.T) {
	resetDB(t)

	timeout := time.Duration(envInt(envTimeoutSec, 120)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	start := time.Now()
	err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	})
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	t.Logf("Invoke completed in %v seconds", time.Since(start).Seconds())

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

func testDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}

func repoRoot() string {
	return filepath.Clean(filepath.Join(testDir(), ".."))
}

func loadSystemPrompt(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var file systemPromptFile
	if err := json.Unmarshal(raw, &file); err != nil {
		return "", err
	}
	if strings.TrimSpace(file.Prompt) == "" {
		return "", fmt.Errorf("%s: empty prompt", path)
	}
	return file.Prompt, nil
}

func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return scanner.Err()
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
