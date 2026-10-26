// Package postgres provides helpers for opening database connections.
package postgres

import (
	"context"
	"fmt"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connection wraps a GORM PostgreSQL handle.
type Connection struct {
	db *gorm.DB
}

// Open connects to PostgreSQL using the given DSN, runs domain auto-migration,
// and returns a Connection.
func Open(dsn string) (*Connection, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Connection{db: db}, nil
}

// migrate creates or updates tables for persisted domain models.
func migrate(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&repolocations.LocationSubtype{},
		&repolocations.Location{},
		&repolocations.Connection{},
		&repomessages.Message{},
		&journal.Request{},
	); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}

	return nil
}

// WithTransaction runs fn inside a database transaction bound to ctx.
// If fn returns an error, the transaction is rolled back; otherwise it is committed.
func (c *Connection) WithTransaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return c.db.WithContext(ctx).Transaction(fn)
}

// DB returns the underlying GORM handle.
func (c *Connection) DB() *gorm.DB {
	return c.db
}
