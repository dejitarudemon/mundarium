package locations

import (
	"github.com/dejitarudemon/mundarium/internal/repositories"
)

// Connection is not a physical place; it represents a directed link between two locations.
type Connection struct {
	ID repositories.ID `gorm:"primaryKey" json:"id"`

	FromID repositories.ID `gorm:"not null;index" json:"from_id"`
	From   *Location       `gorm:"foreignKey:FromID;references:ID;constraint:OnDelete:RESTRICT" json:"from,omitempty"`

	ToID repositories.ID `gorm:"not null;index" json:"to_id"`
	To   *Location       `gorm:"foreignKey:ToID;references:ID;constraint:OnDelete:RESTRICT" json:"to,omitempty"`

	Bidirectional bool `gorm:"not null;default:false" json:"bidirectional"`

	State   repositories.JSON `gorm:"type:jsonb" json:"state"`
	Details repositories.JSON `gorm:"type:jsonb" json:"details"`

	Description string `gorm:"type:text" json:"description"`

	ExpiresAt *int64
}
