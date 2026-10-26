package postgres

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testDSN  string
	testConn *pgstorage.Connection
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

	testDSN = dsn
	testConn = conn
	code = m.Run()
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`TRUNCATE TABLE execution_journal_requests`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func newJournal(t *testing.T) *PostgresJournal {
	t.Helper()
	j, err := NewPostgresJournal(testConn)
	if err != nil {
		t.Fatalf("NewPostgresJournal: %v", err)
	}
	return j
}

func openIsolatedConn(t *testing.T) *pgstorage.Connection {
	t.Helper()
	conn, err := pgstorage.Open(testDSN)
	if err != nil {
		t.Fatalf("open isolated postgres: %v", err)
	}
	t.Cleanup(func() {
		sqlDB, err := conn.DB().DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return conn
}

func closeConn(t *testing.T, conn *pgstorage.Connection) {
	t.Helper()
	sqlDB, err := conn.DB().DB()
	if err != nil {
		t.Fatalf("sql db: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sql db: %v", err)
	}
}
