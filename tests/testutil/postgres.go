package testutil

import (
	"context"
	"fmt"
	"time"

	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// StartPostgres starts a Postgres 16 testcontainer and opens a migrated Connection.
// Caller must Terminate the container.
func StartPostgres(ctx context.Context) (*postgres.PostgresContainer, *pgstorage.Connection, error) {
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
		return nil, nil, fmt.Errorf("start postgres container: %w", err)
	}

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("postgres dsn: %w", err)
	}

	conn, err := pgstorage.Open(dsn)
	if err != nil {
		_ = container.Terminate(ctx)
		return nil, nil, fmt.Errorf("open postgres: %w", err)
	}

	return container, conn, nil
}
