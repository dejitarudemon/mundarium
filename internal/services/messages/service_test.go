package messages_test

import (
	"testing"

	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	"gorm.io/gorm"
)

func TestNewService(t *testing.T) {
	generator := func(db *gorm.DB) *repomessages.MessagesRepository {
		repo, err := repomessages.NewMessagesRepository(db)
		if err != nil {
			t.Fatalf("NewMessagesRepository: %v", err)
		}
		return repo
	}

	t.Run("ok", func(t *testing.T) {
		svc, err := svcmessages.NewService(generator, testConn)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
	})

	t.Run("nil generator", func(t *testing.T) {
		_, err := svcmessages.NewService(nil, testConn)
		if err == nil || err.Error() != "generator is nil" {
			t.Fatalf("want generator is nil, got %v", err)
		}
	})

	t.Run("nil connection", func(t *testing.T) {
		_, err := svcmessages.NewService(generator, nil)
		if err == nil || err.Error() != "connection is nil" {
			t.Fatalf("want connection is nil, got %v", err)
		}
	})
}
