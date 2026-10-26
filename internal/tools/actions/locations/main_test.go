package locations_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"gorm.io/gorm"
)

var (
	testConn *pgstorage.Connection
	testRepo *repolocations.LocationRepository
	testSvc  *svclocations.Service
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	container, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("mundarium"),
		postgres.WithUsername("mundarium"),
		postgres.WithPassword("mundarium"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		os.Exit(1)
	}

	code := 1
	defer func() {
		_ = container.Terminate(ctx)
		os.Exit(code)
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "postgres dsn: %v\n", err)
		return
	}

	conn, err := pgstorage.Open(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open postgres: %v\n", err)
		return
	}

	generator := func(db *gorm.DB) *repolocations.LocationRepository {
		repo, err := repolocations.NewLocationRepository(db, 5, nil)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := svclocations.NewService(generator, conn, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new service: %v\n", err)
		return
	}

	testConn = conn
	testRepo = generator(conn.DB())
	testSvc = svc
	code = m.Run()
}

func resetDB(t *testing.T) {
	t.Helper()

	if err := testConn.DB().Exec(`
		TRUNCATE TABLE connections, locations, location_subtypes
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func definitionsDir(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/tools/actions/locations -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", "definitions"))
}

func createLocationDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "create_location.json")
}

func createSubtypeDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "create_subtype.json")
}

func findSubtypesDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "find_subtypes.json")
}

func findLocationsDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "find_location.json")
}

func moveLocationDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "move_location.json")
}

func updateLocationDetailsDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "update_location_details.json")
}

func updateLocationDescriptionDefinitionPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(definitionsDir(t), "update_location_description.json")
}
