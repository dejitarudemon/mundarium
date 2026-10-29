// Package dices implements the dice application service.
package dices

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
)

// Service coordinates dice rolls against the dices repository.
type Service struct {
	repository *repodices.DicesRepository
	logger     logger.Logger
}

// NewService creates a Service.
// logger may be nil. Returns an error if repository is nil.
func NewService(repository *repodices.DicesRepository, logger logger.Logger) (*Service, error) {
	if repository == nil {
		return nil, errors.New("repository is nil")
	}

	return &Service{
		repository: repository,
		logger:     logger,
	}, nil
}
