// Package locations provides data-access helpers for location domain models.
package locations

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"

	"gorm.io/gorm"
)

// LocationRepository persists and queries Location, LocationSubtype, and Connection records.
type LocationRepository struct {
	db       *gorm.DB
	maxDepth int

	logger logger.Logger
}

// NewLocationRepository creates a LocationRepository backed by db.
// maxDepth is the maximum relative depth accepted by FindLocations (inclusive).
// logger may be nil. Returns an error if db is nil or maxDepth is negative.
func NewLocationRepository(db *gorm.DB, maxDepth int, logger logger.Logger) (*LocationRepository, error) {
	if db == nil {
		return nil, errors.New("expected *gorm.DB, got nil")
	}
	if maxDepth < 0 {
		return nil, errors.New("max depth must be >= 0")
	}

	return &LocationRepository{db: db, maxDepth: maxDepth, logger: logger}, nil
}

// MaxDepth returns the maximum relative depth accepted by FindLocations.
func (r *LocationRepository) MaxDepth() int {
	return r.maxDepth
}
