package move_location_between_parents_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/providers"
	openai "github.com/dejitarudemon/mundarium/internal/providers/implemented/openai"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const (
	villageAName = "Village A"
	villageBName = "Village B"
	tavernName   = "The Copper Fox"
	cellarName   = "Cellar"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip move_location_between_parents Invoke tests: %v\n", reason)
		os.Exit(0)
	}

	prompt, err := testutil.LoadSystemPrompt(filepath.Join(packageDir(), "system_prompt.json"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "load system prompt: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pg, conn, err := testutil.StartPostgres(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	container = pg

	code := 1
	defer func() {
		_ = container.Terminate(ctx)
		os.Exit(code)
	}()

	log := implemented.New(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))

	locGenerator := func(db *gorm.DB) *repolocations.LocationRepository {
		repo, err := repolocations.NewLocationRepository(db, 5, log)
		if err != nil {
			panic(err)
		}
		return repo
	}
	msgGenerator := func(db *gorm.DB) *repomessages.MessagesRepository {
		repo, err := repomessages.NewMessagesRepository(db, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	locSvc, err := svclocations.NewService(locGenerator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new locations service: %v\n", err)
		return
	}
	msgSvc, err := svcmessages.NewService(msgGenerator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new messages service: %v\n", err)
		return
	}

	j, err := pgjournal.NewPostgresJournal(conn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new postgres journal: %v\n", err)
		return
	}

	ex, err := executor.NewExecutor(j, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new executor: %v\n", err)
		return
	}

	root := testutil.RepoRoot()
	role := []types.Role{types.Role_Master}

	findTool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
		return
	}
	moveTool, err := toolslocations.NewMoveLocationTool(
		filepath.Join(root, "definitions", "move_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new move_location tool: %v\n", err)
		return
	}
	sendTool, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"),
		msgSvc,
		nil,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message tool: %v\n", err)
		return
	}
	if err := ex.Register(findTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}
	if err := ex.Register(moveTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register move_location: %v\n", err)
		return
	}
	if err := ex.Register(sendTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register send_message: %v\n", err)
		return
	}

	p, err := openai.NewOpenAICompatibleProvider(
		ex, url, model, apiKey, prompt, types.Role_Master, maxIters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	testRepo = locGenerator(conn.DB())
	provider = p
	code = m.Run()
}

func TestInvoke_MoveLocation_BetweenParents(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	villageA, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        villageAName,
		Description: "western farming village",
		Details:     repositories.JSON(`{"side":"west"}`),
	})
	if err != nil {
		t.Fatalf("seed Village A: %v", err)
	}
	villageB, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        villageBName,
		Description: "eastern market village",
		Details:     repositories.JSON(`{"side":"east"}`),
	})
	if err != nil {
		t.Fatalf("seed Village B: %v", err)
	}

	subtype, err := testRepo.CreateSubtype(ctx, &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"},
		ExpectedTags: pq.StringArray{"village"},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("seed subtype: %v", err)
	}

	const (
		tavernDescription = "A copper-signed tavern by the square."
		tavernDetails     = `{"keeper":"Marta","rooms":4}`
		cellarDescription = "Cool storage under the tavern."
		cellarDetails     = `{"barrels":12}`
	)
	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(villageA.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        tavernName,
		Description: tavernDescription,
		Details:     repositories.JSON(tavernDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	cellar, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(tavern.ID),
		Type:        repolocations.LocationType_ROOM,
		Name:        cellarName,
		Description: cellarDescription,
		Details:     repositories.JSON(cellarDetails),
	})
	if err != nil {
		t.Fatalf("seed cellar: %v", err)
	}

	beforeIDs := map[repositories.ID]bool{
		villageA.ID: true,
		villageB.ID: true,
		tavern.ID:   true,
		cellar.ID:   true,
	}
	tavernBefore := snapshotLocation(t, tavern.ID)
	cellarBefore := snapshotLocation(t, cellar.ID)

	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count before: %v", err)
	}

	userMessage := fmt.Sprintf(
		`Перемести таверну %q (id=%d), которая сейчас в %q (id=%d), в %q (id=%d) через move_location, сохранив подвал id=%d внутри таверны. Затем одним send_message сообщи id таверны и её новый parent_id. Другие локации не создавай.`,
		tavernName, tavern.ID,
		villageAName, villageA.ID,
		villageBName, villageB.ID,
		cellar.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nA=%d B=%d tavern=%d cellar=%d subtype=%d\n",
		villageA.ID, villageB.ID, tavern.ID, cellar.ID, subtype.ID)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(invokeCtx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}

	sendRows := 0
	for _, req := range journaled {
		switch req.Tool {
		case "move_location", "find_locations":
			t.Fatalf("%s is idempotent and must not be journaled, got %#v", req.Tool, req)
		case "create_location", "create_subtype":
			t.Fatalf("%s must not be called, got journal %#v", req.Tool, req)
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendRows == 0 {
		t.Fatal("expected at least one send_message journal row (move_location itself is not journaled)")
	}

	var locationCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter).Error; err != nil {
		t.Fatalf("count after: %v", err)
	}
	if locationCountAfter != locationCountBefore {
		t.Fatalf("no new locations: before=%d after=%d", locationCountBefore, locationCountAfter)
	}

	var all []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("load locations: %v", err)
	}
	for _, loc := range all {
		if !beforeIDs[loc.ID] {
			t.Fatalf("unexpected location id=%d name=%q", loc.ID, loc.Name)
		}
	}

	tavernAfter := snapshotLocation(t, tavern.ID)
	cellarAfter := snapshotLocation(t, cellar.ID)

	if tavernAfter.ParentID != villageB.ID {
		t.Fatalf("tavern parent_id: want %d (Village B), got %d", villageB.ID, tavernAfter.ParentID)
	}
	if cellarAfter.ParentID != tavern.ID {
		t.Fatalf("cellar parent_id: want %d (tavern), got %d", tavern.ID, cellarAfter.ParentID)
	}

	if tavernAfter.Name != tavernBefore.Name ||
		tavernAfter.Description != tavernBefore.Description ||
		tavernAfter.Type != tavernBefore.Type ||
		tavernAfter.SubtypeID != tavernBefore.SubtypeID ||
		!bytes.Equal([]byte(tavernAfter.Details), []byte(tavernBefore.Details)) {
		t.Fatalf("tavern fields mutated beyond parent:\nbefore=%+v\nafter=%+v", tavernBefore, tavernAfter)
	}
	if cellarAfter != cellarBefore {
		t.Fatalf("cellar must be unchanged:\nbefore=%+v\nafter=%+v", cellarBefore, cellarAfter)
	}

	// Search under old/new villages must reflect the move.
	underA, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: villageA.ID,
		Depth:   2,
	})
	if err != nil {
		t.Fatalf("FindLocations Village A: %v", err)
	}
	if containsID(underA, tavern.ID) {
		t.Fatalf("Village A search must not include tavern id=%d; children=%#v", tavern.ID, underA.Children)
	}
	if containsID(underA, cellar.ID) {
		t.Fatalf("Village A search must not include cellar id=%d; children=%#v", cellar.ID, underA.Children)
	}

	underB, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: villageB.ID,
		Depth:   2,
	})
	if err != nil {
		t.Fatalf("FindLocations Village B: %v", err)
	}
	if !containsID(underB, tavern.ID) {
		t.Fatalf("Village B search must include tavern id=%d; children=%#v", tavern.ID, underB.Children)
	}
	if !containsID(underB, cellar.ID) {
		t.Fatalf("Village B search must include cellar id=%d; children=%#v", cellar.ID, underB.Children)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Tavern after ===\n%+v\n", tavernAfter)
	fmt.Printf("=== Cellar after ===\n%+v\n", cellarAfter)
}

type locationSnap struct {
	Name        string
	Description string
	Details     string
	ParentID    repositories.ID
	Type        repolocations.LocationType
	SubtypeID   repositories.ID
}

func snapshotLocation(t *testing.T, id repositories.ID) locationSnap {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	snap := locationSnap{
		Name:        got.Name,
		Description: got.Description,
		Details:     string(got.Details),
		Type:        got.Type,
	}
	if got.ParentID != nil {
		snap.ParentID = *got.ParentID
	}
	if got.SubtypeID != nil {
		snap.SubtypeID = *got.SubtypeID
	}
	return snap
}

func containsID(found repolocations.FindLocationsResult, id repositories.ID) bool {
	for _, level := range found.Children {
		for _, loc := range level {
			if loc.ID == id {
				return true
			}
		}
	}
	return false
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, connections, locations, location_subtypes, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
}

func packageDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(file)
}
