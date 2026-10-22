package postgres_test

import (
	"context"
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/locations"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"gorm.io/gorm"
)

func openConn(t *testing.T) *pgstorage.Connection {
	t.Helper()

	conn, err := pgstorage.Open(testDSN)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return conn
}

func truncate(t *testing.T, db *gorm.DB) {
	t.Helper()

	if err := db.Exec(`
		TRUNCATE TABLE connections, locations, location_subtypes
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate: %v", err)
	}
}

func TestOpen_MigratesTables(t *testing.T) {
	conn := openConn(t)
	db := conn.DB()

	for _, table := range []string{"locations", "location_subtypes", "connections"} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("expected table %q after Open", table)
		}
	}
}

func TestOpen_InvalidDSN(t *testing.T) {
	_, err := pgstorage.Open("host=127.0.0.1 port=1 user=x password=x dbname=x sslmode=disable")
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestDB_ReturnsHandle(t *testing.T) {
	conn := openConn(t)
	if conn.DB() == nil {
		t.Fatal("DB() returned nil")
	}

	sqlDB, err := conn.DB().DB()
	if err != nil {
		t.Fatalf("underlying sql.DB: %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestWithTransaction_Commits(t *testing.T) {
	conn := openConn(t)
	truncate(t, conn.DB())
	ctx := context.Background()

	var id types.ID
	err := conn.WithTransaction(ctx, func(tx *gorm.DB) error {
		loc := &locations.Location{
			ParentID: nil,
			Type:     types.LocationType_REGION,
			Name:     "committed",
		}
		if err := tx.Create(loc).Error; err != nil {
			return err
		}
		id = loc.ID
		return nil
	})
	if err != nil {
		t.Fatalf("WithTransaction: %v", err)
	}

	var got locations.Location
	if err := conn.DB().First(&got, "id = ?", id).Error; err != nil {
		t.Fatalf("load committed row: %v", err)
	}
	if got.Name != "committed" {
		t.Fatalf("want name %q, got %q", "committed", got.Name)
	}
}

func TestWithTransaction_RollsBackOnError(t *testing.T) {
	conn := openConn(t)
	truncate(t, conn.DB())
	ctx := context.Background()

	var id types.ID
	err := conn.WithTransaction(ctx, func(tx *gorm.DB) error {
		loc := &locations.Location{
			ParentID: nil,
			Type:     types.LocationType_REGION,
			Name:     "rolled-back",
		}
		if err := tx.Create(loc).Error; err != nil {
			return err
		}
		id = loc.ID
		return errors.New("force rollback")
	})
	if err == nil || err.Error() != "force rollback" {
		t.Fatalf("want force rollback error, got %v", err)
	}

	var count int64
	if err := conn.DB().Model(&locations.Location{}).Where("id = ?", id).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("want rolled-back insert gone, count=%d", count)
	}
}
