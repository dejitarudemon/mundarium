package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// Connection is not a physical place; it represents a directed link between two locations.
type Connection struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	FromID repositories.ID `gorm:"not null;index" json:"from_id"` // source location ID
	From   *Location       `gorm:"foreignKey:FromID;references:ID;constraint:OnDelete:RESTRICT" json:"from,omitempty"`

	ToID repositories.ID `gorm:"not null;index" json:"to_id"` // destination location ID
	To   *Location       `gorm:"foreignKey:ToID;references:ID;constraint:OnDelete:RESTRICT" json:"to,omitempty"`

	Bidirectional bool `gorm:"not null;default:false" json:"bidirectional"` // whether the link may be traversed in both directions

	State   repositories.JSON `gorm:"type:jsonb" json:"state"`   // runtime/state data for the connection
	Details repositories.JSON `gorm:"type:jsonb" json:"details"` // additional structured metadata

	Description string `gorm:"type:text" json:"description"` // human-readable description of the connection

	ExpiresAt *int64 // optional Unix timestamp when the connection expires; nil means it does not expire
}
