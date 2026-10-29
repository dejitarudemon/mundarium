// Package events implements the event application service.
package events

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"gorm.io/gorm"
)

// EventsRepositoryGenerator builds an EventsRepository bound to the given DB handle.
// Pass the connection root DB or a transaction handle.
type EventsRepositoryGenerator func(*gorm.DB) *repoevents.EventsRepository

// Service coordinates event use cases against storage and execution callbacks.
type Service struct {
	generator  EventsRepositoryGenerator
	connection *postgres.Connection
	logger     logger.Logger
}

// NewService creates a Service.
// logger may be nil. Returns an error if generator or connection is nil.
func NewService(generator EventsRepositoryGenerator, connection *postgres.Connection, logger logger.Logger) (*Service, error) {
	if generator == nil {
		return nil, errors.New("generator is nil")
	}
	if connection == nil {
		return nil, errors.New("connection is nil")
	}

	return &Service{
		generator:  generator,
		connection: connection,
		logger:     logger,
	}, nil
}

// repository returns an EventsRepository using the connection root DB.
func (s *Service) repository() *repoevents.EventsRepository {
	return s.generator(s.connection.DB())
}

// fail records the failure in the execution journal and warns if that recording fails.
func (s *Service) fail(ctx execution.ExecutionContext[journal.Result], cause error, attrs ...any) {
	if err := ctx.Fail(cause, s.connection.DB()); err != nil {
		args := append([]any{"cause", cause, "error", err}, attrs...)
		s.warn("failed to record Fail in journal", args...)
	}
}
