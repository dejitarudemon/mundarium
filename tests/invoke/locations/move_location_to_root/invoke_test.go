package move_location_to_root_test

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
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

const (
	regionName     = "Northern Reach"
	settlementName = "Ashford"
	tavernName     = "The Copper Fox"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	provider  *openai.OpenAICompatibleProvider
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	url, model, apiKey, maxIters, ok, reason := testutil.RequireLLMStudioEnv()
	if !ok {
		fmt.Fprintf(os.Stderr, "skip move_location_to_root Invoke tests: %v\n", reason)
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

	exec, err := executor.NewExecutor(j, log)
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
	if err := exec.Register(findTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register find_locations: %v\n", err)
		return
	}
	if err := exec.Register(moveTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register move_location: %v\n", err)
		return
	}
	if err := exec.Register(sendTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register send_message: %v\n", err)
		return
	}

	p, err := openai.NewOpenAICompatibleProvider(
		exec, url, model, apiKey, prompt, types.Role_Master, maxIters,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new openai provider: %v\n", err)
		return
	}

	testConn = conn
	testRepo = locGenerator(conn.DB())
	provider = p
	ex = exec
	code = m.Run()
}

func TestInvoke_MoveLocation_ToRoot(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	region, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_REGION,
		Name:        regionName,
		Description: "cold northern marches",
		Details:     repositories.JSON(`{"climate":"cold"}`),
	})
	if err != nil {
		t.Fatalf("seed region: %v", err)
	}

	const (
		settlementDescription = "Market town on the river ford."
		settlementDetails     = `{"size":"medium","wall":false}`
		tavernDescription     = "Inn by the ford."
		tavernDetails         = `{"keeper":"Marta"}`
	)
	settlement, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(region.ID),
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        settlementName,
		Description: settlementDescription,
		Details:     repositories.JSON(settlementDetails),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}
	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		Name:        tavernName,
		Description: tavernDescription,
		Details:     repositories.JSON(tavernDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}

	settlementBefore := snapshotLocation(t, settlement.ID)
	tavernBefore := snapshotLocation(t, tavern.ID)
	if settlementBefore.ParentID != region.ID {
		t.Fatalf("precondition: settlement parent want %d, got %d", region.ID, settlementBefore.ParentID)
	}

	var locationCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locationCountBefore).Error; err != nil {
		t.Fatalf("count before: %v", err)
	}

	userMessage := fmt.Sprintf(
		`Сделай поселение %q (id=%d) самостоятельной корневой локацией: вызови move_location с parent_id равным JSON null (не 0 и не omit). Дочерняя таверна id=%d должна остаться внутри поселения. Затем одним send_message подтверди id поселения и что parent_id стал null. Другие локации не создавай.`,
		settlementName, settlement.ID, tavern.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nregion=%d settlement=%d tavern=%d\n", region.ID, settlement.ID, tavern.ID)
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

	assertSettlementIsRoot(t, settlement.ID, settlementBefore)
	assertChildUnchanged(t, tavern.ID, tavernBefore, settlement.ID)
	assertLocationCount(t, locationCountBefore)

	// Region must still exist as its own root.
	regionGot, err := testRepo.GetLocation(ctx, region.ID)
	if err != nil {
		t.Fatalf("GetLocation region: %v", err)
	}
	if regionGot.ParentID != nil {
		t.Fatalf("region must remain root, got parent_id=%v", *regionGot.ParentID)
	}

	afterMove := dumpAll(t)

	// Deterministic JSON-null re-apply: same move again must not change state.
	nullArgs := json.RawMessage(fmt.Sprintf(`{"id":%d,"parent_id":null}`, settlement.ID))
	var decoded struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
	}
	if err := json.Unmarshal(nullArgs, &decoded); err != nil {
		t.Fatalf("unmarshal null args: %v", err)
	}
	if decoded.ID != settlement.ID || decoded.ParentID != nil {
		t.Fatalf("null args decode: want id=%d parent_id=nil, got id=%d parent_id=%v", settlement.ID, decoded.ID, decoded.ParentID)
	}
	if !bytes.Contains(nullArgs, []byte(`"parent_id":null`)) {
		t.Fatalf("raw args must contain JSON null parent_id, got %s", nullArgs)
	}

	raw, err := ex.Execute(ctx, uuid.NewString(), tools.Call{
		ID:   "move-to-root-again",
		Name: "move_location",
		Role: types.Role_Master,
		Args: nullArgs,
	})
	if err != nil {
		t.Fatalf("Executor.Execute move_location parent_id=null: %v", err)
	}
	var moveOut struct {
		ID       repositories.ID  `json:"id"`
		ParentID *repositories.ID `json:"parent_id"`
	}
	if err := json.Unmarshal(raw, &moveOut); err != nil {
		t.Fatalf("unmarshal move result: %v\nraw=%s", err, raw)
	}
	if moveOut.ID != settlement.ID || moveOut.ParentID != nil {
		t.Fatalf("move result: want id=%d parent_id=null, got id=%d parent_id=%v", settlement.ID, moveOut.ID, moveOut.ParentID)
	}

	assertSettlementIsRoot(t, settlement.ID, settlementBefore)
	assertChildUnchanged(t, tavern.ID, tavernBefore, settlement.ID)
	assertLocationCount(t, locationCountBefore)

	afterAgain := dumpAll(t)
	if afterAgain != afterMove {
		t.Fatalf("re-null must not change DB state:\nbefore=%s\nafter=%s", afterMove, afterAgain)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Settlement after ===\n%+v\n", snapshotLocation(t, settlement.ID))
	fmt.Printf("=== Null re-apply result ===\n%s\n", raw)
}

func assertSettlementIsRoot(t *testing.T, id repositories.ID, before locationSnap) {
	t.Helper()
	loc, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	if loc.ParentID != nil {
		t.Fatalf("settlement parent_id: want NULL, got %d", *loc.ParentID)
	}
	got := snapshotLocation(t, id)
	if got.Name != before.Name ||
		got.Description != before.Description ||
		got.Type != before.Type ||
		got.SubtypeID != before.SubtypeID ||
		got.Details != before.Details {
		t.Fatalf("settlement fields mutated beyond parent:\nbefore=%+v\nafter=%+v", before, got)
	}
}

func assertChildUnchanged(t *testing.T, id repositories.ID, before locationSnap, wantParent repositories.ID) {
	t.Helper()
	got := snapshotLocation(t, id)
	if got.ParentID != wantParent {
		t.Fatalf("child parent_id: want %d, got %d", wantParent, got.ParentID)
	}
	if got.Name != before.Name ||
		got.Description != before.Description ||
		got.Type != before.Type ||
		got.SubtypeID != before.SubtypeID ||
		got.Details != before.Details {
		t.Fatalf("child fields mutated:\nbefore=%+v\nafter=%+v", before, got)
	}
}

func assertLocationCount(t *testing.T, want int64) {
	t.Helper()
	var got int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&got).Error; err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if got != want {
		t.Fatalf("location count: want %d, got %d", want, got)
	}
}

func dumpAll(t *testing.T) string {
	t.Helper()
	var all []repolocations.Location
	if err := testConn.DB().Order("id asc").Find(&all).Error; err != nil {
		t.Fatalf("dump locations: %v", err)
	}
	type row struct {
		ID          repositories.ID  `json:"id"`
		ParentID    *repositories.ID `json:"parent_id"`
		Type        string           `json:"type"`
		SubtypeID   *repositories.ID `json:"subtype_id"`
		Name        string           `json:"name"`
		Description string           `json:"description"`
		Details     repositories.JSON `json:"details"`
	}
	rows := make([]row, 0, len(all))
	for _, loc := range all {
		rows = append(rows, row{
			ID:          loc.ID,
			ParentID:    loc.ParentID,
			Type:        loc.Type.String(),
			SubtypeID:   loc.SubtypeID,
			Name:        loc.Name,
			Description: loc.Description,
			Details:     loc.Details,
		})
	}
	raw, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshal dump: %v", err)
	}
	return string(raw)
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
