package dices_test

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	openai "github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svcdices "github.com/dejitarudemon/mundarium/internal/services/dices"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsdices "github.com/dejitarudemon/mundarium/internal/tools/actions/dices"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

// Fixed seed for DicesRepository so roll faces are reproducible across suite runs.
const diceRNGSeed int64 = 42

var (
	testConn     *pgstorage.Connection
	provider     *openai.OpenAICompatibleProvider
	container    *postgres.PostgresContainer
	rollDiceTool *toolsdices.RollDiceTool
	llmURL       string
	llmModel     string
	llmMaxIter   int
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke/dices: %v\n", reason)
		os.Exit(0)
	}
	if maxIters < 12 {
		maxIters = 12
	}
	llmURL, llmModel, llmMaxIter = url, model, maxIters

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

	msgGenerator := func(db *gorm.DB) *repomessages.MessagesRepository {
		repo, err := repomessages.NewMessagesRepository(db, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	diceRepo := repodices.NewDicesRepository(rand.New(rand.NewSource(diceRNGSeed)), log)
	diceSvc, err := svcdices.NewService(diceRepo, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new dices service: %v\n", err)
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
	def := func(name string) string { return filepath.Join(root, "definitions", name) }
	role := []types.Role{types.Role_Master}

	rollTool, err := toolsdices.NewRollDiceTool(def("roll_dices.json"), diceSvc, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new roll_dice tool: %v\n", err)
		return
	}
	if rollTool.IsIdempotent() {
		fmt.Fprintf(os.Stderr, "roll_dice must not be idempotent\n")
		return
	}
	rollDiceTool = rollTool

	sendTool, err := toolsmessages.NewCreateMessageTool(def("send_message.json"), msgSvc, nil, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message tool: %v\n", err)
		return
	}

	for _, tool := range []tools.Tool{recordTool(rollTool), recordTool(sendTool)} {
		if err := ex.Register(tool, role); err != nil {
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
	provider = p

	fmt.Printf("=== invoke/dices config ===\nmodel=%s url=%s max_iters=%d dice_seed=%d\n",
		model, url, maxIters, diceRNGSeed)
	code = m.Run()
}

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}
