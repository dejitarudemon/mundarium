package locations_test

import (
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	"gorm.io/gorm"
)

func TestNewService(t *testing.T) {
	generator := func(db *gorm.DB) *repositories.LocationRepository {
		repo, err := repositories.NewLocationRepository(db)
		if err != nil {
			t.Fatalf("NewLocationRepository: %v", err)
		}
		return repo
	}

	t.Run("ok", func(t *testing.T) {
		svc, err := svclocations.NewService(generator, testConn)
		if err != nil {
			t.Fatalf("NewService: %v", err)
		}
		if svc == nil {
			t.Fatal("expected non-nil service")
		}
	})

	t.Run("nil generator", func(t *testing.T) {
		_, err := svclocations.NewService(nil, testConn)
		if err == nil || err.Error() != "generator is nil" {
			t.Fatalf("want generator is nil, got %v", err)
		}
	})

	t.Run("nil connection", func(t *testing.T) {
		_, err := svclocations.NewService(generator, nil)
		if err == nil || err.Error() != "connection is nil" {
			t.Fatalf("want connection is nil, got %v", err)
		}
	})
}
