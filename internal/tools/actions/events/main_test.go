package events_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	serviceevents "github.com/dejitarudemon/mundarium/internal/services/events"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

var (
	testConn *pgstorage.Connection
	testRepo *repoevents.EventsRepository
	testSvc  *serviceevents.Service
)

func TestMain(m *testing.M) {
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

	generator := func(db *gorm.DB) *repoevents.EventsRepository {
		repo, err := repoevents.NewEventsRepository(db, nil)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := serviceevents.NewService(generator, conn, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new service: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	testSvc = svc
	code = m.Run()
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE event_causals, event_entities, events
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func newExecCtx(t *testing.T) (execution.ExecutionContext[journal.Result], context.CancelFunc) {
	t.Helper()
	complete := func(context.Context, journal.ID, journal.ID, journal.Result, *gorm.DB) error { return nil }
	fail := func(context.Context, journal.ID, journal.ID, error, *gorm.DB) error { return nil }
	ctx, cancel, err := execution.NewExecutionContext(
		context.Background(),
		journal.ID("test-request"),
		journal.ID("test-group"),
		types.Role_Master,
		complete,
		fail,
	)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ctx, cancel
}

func definitionsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "definitions"))
}

func defPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), name)
}
