// Package messages implements the message application service.
package messages

import (
	"errors"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/libs/logger"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"gorm.io/gorm"
)

// MessagesRepositoryGenerator builds a MessagesRepository bound to the given DB handle.
// Pass the connection root DB or a transaction handle.
type MessagesRepositoryGenerator func(*gorm.DB) *repomessages.MessagesRepository

// Service coordinates message use cases against storage and execution callbacks.
//
// Idempotency (for tool / executor wiring):
//   - CreateMessage is not idempotent: each call inserts a new row.
//   - GetMessage, GetMessages, SetDeliveredAt, and DeleteMessages are idempotent.
type Service struct {
	generator  MessagesRepositoryGenerator
	connection *postgres.Connection
	logger     logger.Logger
}

// NewService creates a Service.
// logger may be nil. Returns an error if generator or connection is nil.
func NewService(generator MessagesRepositoryGenerator, connection *postgres.Connection, logger logger.Logger) (*Service, error) {
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

// repository returns a MessagesRepository using the connection root DB.
func (s *Service) repository() *repomessages.MessagesRepository {
	return s.generator(s.connection.DB())
}

// fail records the failure in the execution journal and warns if that recording fails.
func (s *Service) fail(ctx execution.ExecutionContext[journal.Result], cause error, attrs ...any) {
	if err := ctx.Fail(cause, s.connection.DB()); err != nil {
		args := append([]any{"cause", cause, "error", err}, attrs...)
		s.warn("failed to record Fail in journal", args...)
	}
}
