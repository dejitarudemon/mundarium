package find_locations_search_filters_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
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

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip find_locations_search_filters Invoke tests: %v\n", reason)
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
	findTool, err := toolslocations.NewFindLocationsTool(
		filepath.Join(root, "definitions", "find_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_locations tool: %v\n", err)
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
	role := []types.Role{types.Role_Master}
	if err := ex.Register(findTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
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

func TestInvoke_FindLocations_SearchFilters(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	seed := seedSearchTree(t)

	// Deterministic contract checks against the same repository the tool uses.
	assertFindContract(t, ctx, seed)

	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count locations before: %v", err)
	}

	userMessage := fmt.Sprintf(
		`Выполни последовательно find_locations по scope_id=%d:
1) label=query_ember: query="ember", depth=2
2) label=type_structure_d1: type="structure", depth=1
3) label=subtype_tavern: subtype=%d, depth=2
4) label=depth0: depth=0
5) label=depth1: depth=1
6) label=depth2: depth=2
Затем одним send_message перечисли для каждого label найденные id (включая scope, если он в ответе инструмента). Другие тулзы не вызывай.`,
		seed.settlement.ID,
		seed.tavernSubtype.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 240)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nsettlement=%d forge=%d tavern=%d cellar=%d guard=%d stall=%d\n",
		seed.settlement.ID, seed.forge.ID, seed.tavern.ID, seed.cellar.ID, seed.guard.ID, seed.stall.ID)
	fmt.Printf("=== Invoke input ===\ninvoke_id: %s\nuser: %s\n", invokeID, userMessage)

	if err := provider.Invoke(invokeCtx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	}); err != nil {
		t.Fatalf("Invoke: %v", err)
	}

	var locationCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountAfter).Error; err != nil {
		t.Fatalf("count locations after: %v", err)
	}
	if locationCountAfter != locationCountBefore {
		t.Fatalf("DB must not change: before=%d after=%d", locationCountBefore, locationCountAfter)
	}

	var journaled []journal.Request
	if err := testConn.DB().
		Where("group_id = ?", invokeID).
		Order("created_at asc").
		Find(&journaled).Error; err != nil {
		t.Fatalf("load journal: %v", err)
	}
	for _, req := range journaled {
		if req.Tool == "find_locations" {
			t.Fatalf("find_locations must not be journaled, got %#v", req)
		}
	}
	var sendRows int
	for _, req := range journaled {
		if req.Tool == "send_message" {
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendRows == 0 {
		t.Fatal("expected send_message with find results")
	}

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected message text")
	}
	text := messages[len(messages)-1].Text
	fmt.Printf("=== Player message ===\n%s\n", text)

	mustContainID(t, text, "query_ember→forge", seed.forge.ID)
	mustContainID(t, text, "type_structure→forge", seed.forge.ID)
	mustContainID(t, text, "type_structure→tavern", seed.tavern.ID)
	mustContainID(t, text, "type_structure→guard", seed.guard.ID)
	mustContainID(t, text, "subtype_tavern→tavern", seed.tavern.ID)
	mustContainID(t, text, "depth0/1/2→settlement scope", seed.settlement.ID)
	mustContainID(t, text, "depth2→cellar", seed.cellar.ID)
}

type searchTree struct {
	settlement    *repolocations.Location
	forge         *repolocations.Location
	tavern        *repolocations.Location
	cellar        *repolocations.Location
	guard         *repolocations.Location
	stall         *repolocations.Location
	tavernSubtype *repolocations.LocationSubtype
	smithySubtype *repolocations.LocationSubtype
	towerSubtype  *repolocations.LocationSubtype
}

func seedSearchTree(t *testing.T) searchTree {
	t.Helper()

	settlement, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "Riverford",
		Description: "riverside settlement used as search scope",
		Details:     repositories.JSON(`{"scope":true}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	smithy, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key: "smithy", Type: repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"craft"}, ExpectedTags: pq.StringArray{"town"}, Description: "forge",
	})
	if err != nil {
		t.Fatalf("seed smithy subtype: %v", err)
	}
	tavernST, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key: "tavern", Type: repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"}, ExpectedTags: pq.StringArray{"town"}, Description: "inn",
	})
	if err != nil {
		t.Fatalf("seed tavern subtype: %v", err)
	}
	tower, err := testRepo.CreateSubtype(context.Background(), &repolocations.LocationSubtype{
		Key: "watchtower", Type: repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"watch"}, ExpectedTags: pq.StringArray{"wall"}, Description: "tower",
	})
	if err != nil {
		t.Fatalf("seed tower subtype: %v", err)
	}

	// "ember" only in description, not in name.
	forge, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(settlement.ID), Type: repolocations.LocationType_STRUCTURE, SubtypeID: new(smithy.ID),
		Name: "Ironworks", Description: "An ember-lit forge behind the market.", Details: repositories.JSON(`{"hot":true}`),
	})
	if err != nil {
		t.Fatalf("seed forge: %v", err)
	}
	tavern, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(settlement.ID), Type: repolocations.LocationType_STRUCTURE, SubtypeID: new(tavernST.ID),
		Name: "Old Barrel", Description: "noisy tavern", Details: repositories.JSON(`{"ale":true}`),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	cellar, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(tavern.ID), Type: repolocations.LocationType_ROOM,
		Name: "Beer Cellar", Description: "cool storage under the tavern", Details: repositories.JSON(`{"casks":12}`),
	})
	if err != nil {
		t.Fatalf("seed cellar: %v", err)
	}
	guard, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(settlement.ID), Type: repolocations.LocationType_STRUCTURE, SubtypeID: new(tower.ID),
		Name: "North Gate Post", Description: "wooden watch post", Details: repositories.JSON(`{"guards":2}`),
	})
	if err != nil {
		t.Fatalf("seed guard: %v", err)
	}
	stall, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		ParentID: new(settlement.ID), Type: repolocations.LocationType_SITE,
		Name: "Fish Stall", Description: "morning market stall", Details: repositories.JSON(`{"goods":"fish"}`),
	})
	if err != nil {
		t.Fatalf("seed stall: %v", err)
	}

	return searchTree{
		settlement: settlement, forge: forge, tavern: tavern, cellar: cellar, guard: guard, stall: stall,
		tavernSubtype: tavernST, smithySubtype: smithy, towerSubtype: tower,
	}
}

func assertFindContract(t *testing.T, ctx context.Context, seed searchTree) {
	t.Helper()

	ember := "ember"
	byQuery, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: seed.settlement.ID, Query: &ember, Depth: 2,
	})
	if err != nil {
		t.Fatalf("find query ember: %v", err)
	}
	assertScope(t, byQuery, seed.settlement.ID)
	assertChildrenIDs(t, byQuery, map[repositories.ID]bool{seed.forge.ID: true})
	assertNotInChildren(t, byQuery, seed.tavern.ID, seed.cellar.ID, seed.guard.ID, seed.stall.ID)

	structure := repolocations.LocationType_STRUCTURE
	byTypeD1, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: seed.settlement.ID, Type: &structure, Depth: 1,
	})
	if err != nil {
		t.Fatalf("find type structure depth1: %v", err)
	}
	assertScope(t, byTypeD1, seed.settlement.ID)
	assertChildrenIDs(t, byTypeD1, map[repositories.ID]bool{
		seed.forge.ID: true, seed.tavern.ID: true, seed.guard.ID: true,
	})
	assertNotInChildren(t, byTypeD1, seed.cellar.ID, seed.stall.ID)

	bySubtype, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: seed.settlement.ID, SubtypeID: new(seed.tavernSubtype.ID), Depth: 2,
	})
	if err != nil {
		t.Fatalf("find subtype tavern: %v", err)
	}
	assertScope(t, bySubtype, seed.settlement.ID)
	assertChildrenIDs(t, bySubtype, map[repositories.ID]bool{seed.tavern.ID: true})
	assertNotInChildren(t, bySubtype, seed.forge.ID, seed.cellar.ID, seed.guard.ID, seed.stall.ID)

	d0, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{ScopeID: seed.settlement.ID, Depth: 0})
	if err != nil {
		t.Fatalf("find depth0: %v", err)
	}
	assertScope(t, d0, seed.settlement.ID)
	if len(d0.Children) != 0 {
		t.Fatalf("depth=0 must not include descendants, got %#v", d0.Children)
	}

	d1, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{ScopeID: seed.settlement.ID, Depth: 1})
	if err != nil {
		t.Fatalf("find depth1: %v", err)
	}
	assertScope(t, d1, seed.settlement.ID)
	assertChildrenIDs(t, d1, map[repositories.ID]bool{
		seed.forge.ID: true, seed.tavern.ID: true, seed.guard.ID: true, seed.stall.ID: true,
	})
	assertNotInChildren(t, d1, seed.cellar.ID)

	d2, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{ScopeID: seed.settlement.ID, Depth: 2})
	if err != nil {
		t.Fatalf("find depth2: %v", err)
	}
	assertScope(t, d2, seed.settlement.ID)
	assertChildrenIDs(t, d2, map[repositories.ID]bool{
		seed.forge.ID: true, seed.tavern.ID: true, seed.guard.ID: true, seed.stall.ID: true, seed.cellar.ID: true,
	})
}

func assertScope(t *testing.T, got repolocations.FindLocationsResult, want repositories.ID) {
	t.Helper()
	if got.Scope.ID != want {
		t.Fatalf("scope: want %d, got %d", want, got.Scope.ID)
	}
}

func assertChildrenIDs(t *testing.T, got repolocations.FindLocationsResult, want map[repositories.ID]bool) {
	t.Helper()
	found := childIDSet(got)
	for id := range want {
		if !found[id] {
			t.Fatalf("missing child id=%d in %#v", id, found)
		}
	}
	for id := range found {
		if !want[id] {
			t.Fatalf("unexpected child id=%d; want %#v", id, want)
		}
	}
}

func assertNotInChildren(t *testing.T, got repolocations.FindLocationsResult, ids ...repositories.ID) {
	t.Helper()
	found := childIDSet(got)
	for _, id := range ids {
		if found[id] {
			t.Fatalf("id=%d must not appear in children %#v", id, found)
		}
	}
}

func childIDSet(got repolocations.FindLocationsResult) map[repositories.ID]bool {
	out := map[repositories.ID]bool{}
	for _, level := range got.Children {
		for _, loc := range level {
			out[loc.ID] = true
		}
	}
	return out
}

func mustContainID(t *testing.T, text, label string, id repositories.ID) {
	t.Helper()
	idStr := strconv.FormatUint(uint64(id), 10)
	if !strings.Contains(text, idStr) {
		t.Fatalf("%s: message must contain id=%s, got %q", label, idStr, text)
	}
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
