// Package postgres provides helpers for opening database connections.
package postgres

import (
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// Connection wraps a GORM PostgreSQL handle.
type Connection struct {
	db *gorm.DB
}

// Open connects to PostgreSQL using the given DSN, runs locations auto-migration,
// and returns a Connection.
func Open(dsn string) (*Connection, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}

	if err := migrateLocations(db); err != nil {
		return nil, err
	}

	return &Connection{db: db}, nil
}

// migrateLocations creates or updates tables for location domain models.
func migrateLocations(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&locations.LocationSubtype{},
		&locations.Location{},
		&locations.Connection{},
	); err != nil {
		return fmt.Errorf("migrate locations: %w", err)
	}

	return nil
}

// WithTransaction runs fn inside a database transaction.
// If fn returns an error, the transaction is rolled back; otherwise it is committed.
func (c *Connection) WithTransaction(fn func(tx *gorm.DB) error) error {
	return c.db.Transaction(fn)
}
