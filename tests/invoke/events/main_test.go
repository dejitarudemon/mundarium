package events_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	openai "github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn   *pgstorage.Connection
	testRepo   *repoevents.EventsRepository
	provider   *openai.OpenAICompatibleProvider
	container  *postgres.PostgresContainer
	llmURL     string
	llmModel   string
	llmMaxIter int
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip invoke/events: %v\n", reason)
		os.Exit(0)
	}
	if maxIters < 16 {
		maxIters = 16
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

	evGenerator := func(db *gorm.DB) *repoevents.EventsRepository {
		repo, err := repoevents.NewEventsRepository(db, log)
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

	evSvc, err := svcevents.NewService(evGenerator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new events service: %v\n", err)
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
	def := func(name string) string { return filepath.Join(root, "definitions", name) }

	makers := []func() (tools.Tool, error){
		func() (tools.Tool, error) { return toolsevents.NewCreateEventTool(def("create_event.json"), evSvc, log) },
		func() (tools.Tool, error) { return toolsevents.NewFindEventsTool(def("find_events.json"), evSvc, log) },
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventSummaryTool(def("update_event_summary.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventDetailsTool(def("update_event_details.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventImportanceTool(def("update_event_importance.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventLocationTool(def("update_event_location.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewCreateEventEntityTool(def("create_event_entity.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewFindEventEntitiesTool(def("find_event_entities.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityRoleTool(def("update_event_entity_role.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityKnownAtTool(def("update_event_entity_known_at.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityBeliefTool(def("update_event_entity_belief.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityDetailsTool(def("update_event_entity_details.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewCreateEventCausalTool(def("create_event_causal.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewFindEventCausalsTool(def("find_event_causals.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventCausalCauseTool(def("update_event_causal_cause.json"), evSvc, log)
		},
		func() (tools.Tool, error) {
			return toolsmessages.NewCreateMessageTool(def("send_message.json"), msgSvc, nil, log)
		},
	}

	for _, makeTool := range makers {
		tool, err := makeTool()
		if err != nil {
			fmt.Fprintf(os.Stderr, "new tool: %v\n", err)
			return
		}
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
	testRepo = evGenerator(conn.DB())
	provider = p

	fmt.Printf("=== invoke/events config ===\nmodel=%s url=%s max_iters=%d\n", model, url, maxIters)
	code = m.Run()
}

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}
