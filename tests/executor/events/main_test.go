package events_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolsevents "github.com/dejitarudemon/mundarium/internal/tools/actions/events"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repoevents.EventsRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// Deterministic Executor E2E: real Postgres, real tools/service/repo. No LLM.
	_ = testutil.LoadDotEnvFromRepo()

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

	generator := func(db *gorm.DB) *repoevents.EventsRepository {
		repo, err := repoevents.NewEventsRepository(db, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := svcevents.NewService(generator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new events service: %v\n", err)
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
	def := func(name string) string { return filepath.Join(root, "definitions", name) }

	toolsToRegister := []struct {
		name string
		make func() (tools.Tool, error)
	}{
		{"create_event", func() (tools.Tool, error) {
			return toolsevents.NewCreateEventTool(def("create_event.json"), svc, log)
		}},
		{"find_events", func() (tools.Tool, error) {
			return toolsevents.NewFindEventsTool(def("find_events.json"), svc, log)
		}},
		{"update_event_summary", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventSummaryTool(def("update_event_summary.json"), svc, log)
		}},
		{"update_event_details", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventDetailsTool(def("update_event_details.json"), svc, log)
		}},
		{"update_event_importance", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventImportanceTool(def("update_event_importance.json"), svc, log)
		}},
		{"update_event_location", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventLocationTool(def("update_event_location.json"), svc, log)
		}},
		{"create_event_entity", func() (tools.Tool, error) {
			return toolsevents.NewCreateEventEntityTool(def("create_event_entity.json"), svc, log)
		}},
		{"find_event_entities", func() (tools.Tool, error) {
			return toolsevents.NewFindEventEntitiesTool(def("find_event_entities.json"), svc, log)
		}},
		{"update_event_entity_role", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityRoleTool(def("update_event_entity_role.json"), svc, log)
		}},
		{"update_event_entity_known_at", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityKnownAtTool(def("update_event_entity_known_at.json"), svc, log)
		}},
		{"update_event_entity_belief", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityBeliefTool(def("update_event_entity_belief.json"), svc, log)
		}},
		{"update_event_entity_details", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventEntityDetailsTool(def("update_event_entity_details.json"), svc, log)
		}},
		{"create_event_causal", func() (tools.Tool, error) {
			return toolsevents.NewCreateEventCausalTool(def("create_event_causal.json"), svc, log)
		}},
		{"find_event_causals", func() (tools.Tool, error) {
			return toolsevents.NewFindEventCausalsTool(def("find_event_causals.json"), svc, log)
		}},
		{"update_event_causal_cause", func() (tools.Tool, error) {
			return toolsevents.NewUpdateEventCausalCauseTool(def("update_event_causal_cause.json"), svc, log)
		}},
	}

	for _, item := range toolsToRegister {
		tool, err := item.make()
		if err != nil {
			fmt.Fprintf(os.Stderr, "new %s: %v\n", item.name, err)
			return
		}
		if err := exec.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", item.name, err)
			return
		}
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}
