package events_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

var (
	testDB   *gorm.DB
	testRepo *repoevents.EventsRepository
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

	repo, err := repoevents.NewEventsRepository(conn.DB(), nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new repository: %v\n", err)
		return
	}

	testDB = conn.DB()
	testRepo = repo
	code = m.Run()
}

func resetDB(t *testing.T) {
	t.Helper()

	if err := testDB.Exec(`
		TRUNCATE TABLE event_causals, event_entities, events
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}
