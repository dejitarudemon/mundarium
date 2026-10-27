package events

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// ID uniquely identifies an event.
type ID uuid.UUID

// New returns a new random event ID.
func New() ID {
	return ID(uuid.New())
}

// String returns the canonical UUID string form of the ID.
func (id ID) String() string {
	return uuid.UUID(id).String()
}

// MarshalJSON encodes the ID as a UUID string.
func (id ID) MarshalJSON() ([]byte, error) {
	return json.Marshal(uuid.UUID(id).String())
}

// UnmarshalJSON decodes a UUID string into an ID.
func (id *ID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("events.ID: %w", err)
	}
	u, err := uuid.Parse(s)
	if err != nil {
		return fmt.Errorf("events.ID: %w", err)
	}
	*id = ID(u)
	return nil
}

// Value implements driver.Valuer for database persistence.
func (id ID) Value() (driver.Value, error) {
	return uuid.UUID(id).Value()
}

// Scan implements sql.Scanner for database reads.
func (id *ID) Scan(src any) error {
	var u uuid.UUID
	if err := u.Scan(src); err != nil {
		return fmt.Errorf("events.ID: %w", err)
	}
	*id = ID(u)
	return nil
}
