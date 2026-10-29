package events_test

import (
	"testing"

	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	svcevents "github.com/dejitarudemon/mundarium/internal/services/events"
	"gorm.io/gorm"
)

func TestNewService(t *testing.T) {
	generator := func(db *gorm.DB) *repoevents.EventsRepository {
		repo, err := repoevents.NewEventsRepository(db, nil)
		if err != nil {
			t.Fatalf("NewEventsRepository: %v", err)
		}
		return repo
	}

	t.Run("ok", func(t *testing.T) {
		svc, err := svcevents.NewService(generator, testConn, nil)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
	})

	t.Run("nil generator", func(t *testing.T) {
		_, err := svcevents.NewService(nil, testConn, nil)
		if err == nil || err.Error() != "generator is nil" {
			t.Fatalf("want generator is nil, got %v", err)
		}
	})

	t.Run("nil connection", func(t *testing.T) {
		_, err := svcevents.NewService(generator, nil, nil)
		if err == nil || err.Error() != "connection is nil" {
			t.Fatalf("want connection is nil, got %v", err)
		}
	})
}
