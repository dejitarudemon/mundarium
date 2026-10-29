package entities

import (
	"database/sql/driver"
	"fmt"

	"github.com/google/uuid"
)

// ID uniquely identifies an entity in the world model.
type ID uuid.UUID

// New returns a new random entity ID.
func New() ID {
	return ID(uuid.New())
}

// String returns the canonical UUID string form of the ID.
func (id ID) String() string {
	return uuid.UUID(id).String()
}

// Value implements driver.Valuer for database persistence.
func (id ID) Value() (driver.Value, error) {
	return uuid.UUID(id).Value()
}

// Scan implements sql.Scanner for database reads.
func (id *ID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return fmt.Errorf("entities.ID: %w", err)
	}
	*id = ID(u)
	return nil
}
