// Package locations implements the location application service.
package locations

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"gorm.io/gorm"
)

// LocationRepositoryGenerator builds a LocationRepository bound to the given DB handle.
// Pass the connection root DB or a transaction handle.
type LocationRepositoryGenerator func(*gorm.DB) *repositories.LocationRepository

// Service coordinates location use cases against storage and execution callbacks.
type Service struct {
	generator  LocationRepositoryGenerator
	connection *postgres.Connection
}

// NewService creates a Service.
// Returns an error if generator or connection is nil.
func NewService(generator LocationRepositoryGenerator, connection *postgres.Connection) (*Service, error) {
	if generator == nil {
		return nil, errors.New("generator is nil")
	}

	if connection == nil {
		return nil, errors.New("connection is nil")
	}

	return &Service{
		generator:  generator,
		connection: connection,
	}, nil
}

// repository returns a LocationRepository using the connection root DB.
func (s *Service) repository() *repositories.LocationRepository {
	return s.generator(s.connection.DB())
}
