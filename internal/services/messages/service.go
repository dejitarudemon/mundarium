// Package messages implements the message application service.
package messages

import (
	"errors"

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
}

// NewService creates a Service.
// Returns an error if generator or connection is nil.
func NewService(generator MessagesRepositoryGenerator, connection *postgres.Connection) (*Service, error) {
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

// repository returns a MessagesRepository using the connection root DB.
func (s *Service) repository() *repomessages.MessagesRepository {
	return s.generator(s.connection.DB())
}
