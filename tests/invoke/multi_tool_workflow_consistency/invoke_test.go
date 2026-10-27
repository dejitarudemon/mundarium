package multi_tool_workflow_consistency_test

import (
	"bytes"
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
	"github.com/dejitarudemon/mundarium/internal/tools"
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
	worldName      = "Test World"
	settlementName = "Ashford"
	tavernName     = "The Copper Fox"
	cellarName     = "Cellar"
	storageName    = "Storage Closet"

	oldDescription = "A copper-signed tavern by the market square."
	newDescription = "Lantern light spills onto the cobbles; the Copper Fox smells of spice and wet wool."
	oldDetails     = `{"keeper":"Marta","rooms":4,"rumor":"A map is hidden beneath the hearth"}`
	wantDetailsRaw = `{"keeper":"Edwin","rooms":5}`
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
		fmt.Fprintf(os.Stderr, "skip multi_tool_workflow_consistency Invoke tests: %v\n", reason)
		os.Exit(0)
	}
	if maxIters < 12 {
		maxIters = 12
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
	defs := []struct {
		name string
		make func() (tools.Tool, error)
	}{
		{"find_locations", func() (tools.Tool, error) {
			return toolslocations.NewFindLocationsTool(filepath.Join(root, "definitions", "find_location.json"), locSvc, log)
		}},
		{"find_subtypes", func() (tools.Tool, error) {
			return toolslocations.NewFindSubtypesTool(filepath.Join(root, "definitions", "find_subtypes.json"), locSvc, log)
		}},
		{"create_subtype", func() (tools.Tool, error) {
			return toolslocations.NewCreateSubtypeTool(filepath.Join(root, "definitions", "create_subtype.json"), locSvc, log)
		}},
		{"create_location", func() (tools.Tool, error) {
			return toolslocations.NewCreateLocationTool(filepath.Join(root, "definitions", "create_location.json"), locSvc, log)
		}},
		{"update_location_description", func() (tools.Tool, error) {
			return toolslocations.NewUpdateLocationDescriptionTool(filepath.Join(root, "definitions", "update_location_description.json"), locSvc, log)
		}},
		{"update_location_details", func() (tools.Tool, error) {
			return toolslocations.NewUpdateLocationDetailsTool(filepath.Join(root, "definitions", "update_location_details.json"), locSvc, log)
		}},
		{"send_message", func() (tools.Tool, error) {
			return toolsmessages.NewCreateMessageTool(filepath.Join(root, "definitions", "send_message.json"), msgSvc, nil, log)
		}},
	}
	for _, d := range defs {
		tool, err := d.make()
		if err != nil {
			fmt.Fprintf(os.Stderr, "new %s: %v\n", d.name, err)
			return
		}
		if err := ex.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", d.name, err)
			return
		}
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

func TestInvoke_MultiToolWorkflow_Consistency(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	world, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_WORLD,
		Name:        worldName,
		Description: "seeded world",
		Details:     repositories.JSON(`{"seed":true}`),
	})
	if err != nil {
		t.Fatalf("seed world: %v", err)
	}
	settlement, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(world.ID),
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        settlementName,
		Description: "market town",
		Details:     repositories.JSON(`{"size":"medium"}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	tavernSubtype, err := testRepo.CreateSubtype(ctx, &repolocations.LocationSubtype{
		Key:          "tavern",
		Type:         repolocations.LocationType_STRUCTURE,
		DefiningTags: pq.StringArray{"rest"},
		ExpectedTags: pq.StringArray{"village"},
		Description:  "inn",
	})
	if err != nil {
		t.Fatalf("seed tavern subtype: %v", err)
	}
	storageSubtype, err := testRepo.CreateSubtype(ctx, &repolocations.LocationSubtype{
		Key:          "storage",
		Type:         repolocations.LocationType_ROOM,
		DefiningTags: pq.StringArray{"storage"},
		ExpectedTags: pq.StringArray{"supplies"},
		Description:  "storage room",
	})
	if err != nil {
		t.Fatalf("seed storage subtype: %v", err)
	}

	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(tavernSubtype.ID),
		Name:        tavernName,
		Description: oldDescription,
		Details:     repositories.JSON(oldDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}
	cellar, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(tavern.ID),
		Type:        repolocations.LocationType_ROOM,
		Name:        cellarName,
		Description: "Cool storage under the tavern.",
		Details:     repositories.JSON(`{"barrels":12}`),
	})
	if err != nil {
		t.Fatalf("seed cellar: %v", err)
	}

	worldBefore := snapshotMeta(t, world.ID)
	settlementBefore := snapshotMeta(t, settlement.ID)
	cellarBefore := snapshotFull(t, cellar.ID)
	tavernMetaBefore := snapshotMeta(t, tavern.ID)

	var locCountBefore, subtypeCountBefore int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locCountBefore).Error; err != nil {
		t.Fatalf("count locations: %v", err)
	}
	if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&subtypeCountBefore).Error; err != nil {
		t.Fatalf("count subtypes: %v", err)
	}

	userMessage := fmt.Sprintf(
		`В мире scope_id=%d найди таверну %q (ожидаемый id=%d) через find_locations.
Затем для этой же записи:
1) update_location_description на точный текст %q
2) update_location_details full overwrite на %s
Создай внутри таверны новую комнату хранения с именем %q (type=room). Сначала find_subtypes; предпочтительно subtype id=%d (key=storage), иначе subtype=null или создай переиспользуемый room-подтип. Существующий подвал id=%d (%q) не дублируй и не перемещай.
Другие локации (мир id=%d, поселение id=%d) не меняй.
В конце одним send_message сообщи id таверны, что описание/детали обновлены, id новой комнаты и что подвал на месте.`,
		settlement.ID,
		tavernName, tavern.ID,
		newDescription,
		wantDetailsRaw,
		storageName, storageSubtype.ID,
		cellar.ID, cellarName,
		world.ID, settlement.ID,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 300)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\nworld=%d settlement=%d tavern=%d cellar=%d tavern_subtype=%d storage_subtype=%d\n",
		world.ID, settlement.ID, tavern.ID, cellar.ID, tavernSubtype.ID, storageSubtype.ID)
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

	var createLocationRows, createSubtypeRows, sendMessageRows int
	for _, req := range journaled {
		switch req.Tool {
		case "find_locations", "find_subtypes", "update_location_description", "update_location_details":
			t.Fatalf("%s is idempotent and must not be journaled, got %#v", req.Tool, req)
		case "create_location":
			createLocationRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("create_location %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		case "create_subtype":
			createSubtypeRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("create_subtype %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		case "send_message":
			sendMessageRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if createLocationRows != 1 {
		t.Fatalf("want exactly 1 create_location (storage room), got %d", createLocationRows)
	}
	if sendMessageRows == 0 {
		t.Fatal("expected send_message journal row")
	}
	// Prefer reusing seeded storage subtype; creating another is allowed but should be rare.
	var subtypeCountAfter int64
	if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&subtypeCountAfter).Error; err != nil {
		t.Fatalf("count subtypes after: %v", err)
	}
	if subtypeCountAfter < subtypeCountBefore || subtypeCountAfter > subtypeCountBefore+int64(createSubtypeRows) {
		t.Fatalf("subtype count inconsistent: before=%d after=%d create_subtype_rows=%d",
			subtypeCountBefore, subtypeCountAfter, createSubtypeRows)
	}

	// Tavern updates on the same row.
	gotTavern, err := testRepo.GetLocation(ctx, tavern.ID)
	if err != nil {
		t.Fatalf("GetLocation tavern: %v", err)
	}
	if gotTavern.Description != newDescription {
		t.Fatalf("tavern description: want %q, got %q", newDescription, gotTavern.Description)
	}
	if strings.Contains(gotTavern.Description, oldDescription) {
		t.Fatalf("old description must be replaced, got %q", gotTavern.Description)
	}
	assertDetailsEqual(t, wantDetailsRaw, gotTavern.Details)
	tavernMetaAfter := snapshotMeta(t, tavern.ID)
	if tavernMetaAfter.Name != tavernMetaBefore.Name ||
		tavernMetaAfter.ParentID != tavernMetaBefore.ParentID ||
		tavernMetaAfter.Type != tavernMetaBefore.Type ||
		tavernMetaAfter.SubtypeID != tavernMetaBefore.SubtypeID {
		t.Fatalf("tavern identity fields mutated:\nbefore=%+v\nafter=%+v", tavernMetaBefore, tavernMetaAfter)
	}

	// Cellar preserved.
	cellarAfter := snapshotFull(t, cellar.ID)
	if cellarAfter != cellarBefore {
		t.Fatalf("cellar must be unchanged:\nbefore=%+v\nafter=%+v", cellarBefore, cellarAfter)
	}
	var cellarDupes int64
	if err := testConn.DB().Model(&repolocations.Location{}).
		Where("parent_id = ? AND name = ?", tavern.ID, cellarName).
		Count(&cellarDupes).Error; err != nil {
		t.Fatalf("count cellar dupes: %v", err)
	}
	if cellarDupes != 1 {
		t.Fatalf("want exactly one cellar under tavern, got %d", cellarDupes)
	}

	// New storage room under tavern.
	var storageRooms []repolocations.Location
	if err := testConn.DB().
		Where("parent_id = ? AND id <> ?", tavern.ID, cellar.ID).
		Order("id asc").
		Find(&storageRooms).Error; err != nil {
		t.Fatalf("load new children: %v", err)
	}
	if len(storageRooms) != 1 {
		t.Fatalf("want exactly one new child under tavern besides cellar, got %d (%v)",
			len(storageRooms), names(storageRooms))
	}
	storage := storageRooms[0]
	if storage.Type != repolocations.LocationType_ROOM {
		t.Fatalf("storage type: want room, got %s", storage.Type.String())
	}
	if storage.ParentID == nil || *storage.ParentID != tavern.ID {
		t.Fatalf("storage parent: want %d, got %v", tavern.ID, storage.ParentID)
	}
	if storage.SubtypeID != nil && *storage.SubtypeID != storageSubtype.ID {
		// Model may have created a new subtype; that is fine if journaled.
		var st repolocations.LocationSubtype
		if err := testConn.DB().First(&st, *storage.SubtypeID).Error; err != nil {
			t.Fatalf("storage subtype id=%d missing: %v", *storage.SubtypeID, err)
		}
		if st.Type != repolocations.LocationType_ROOM {
			t.Fatalf("storage subtype type: want room, got %s", st.Type.String())
		}
	}

	var locCountAfter int64
	if err := testConn.DB().Model(&repolocations.Location{}).Count(&locCountAfter).Error; err != nil {
		t.Fatalf("count locations after: %v", err)
	}
	if locCountAfter != locCountBefore+1 {
		t.Fatalf("location count: want %d (only storage room added), got %d", locCountBefore+1, locCountAfter)
	}

	if snapshotMeta(t, world.ID) != worldBefore {
		t.Fatalf("world mutated")
	}
	if snapshotMeta(t, settlement.ID) != settlementBefore {
		t.Fatalf("settlement mutated")
	}

	// Deterministic find still surfaces tavern under settlement.
	found, err := testRepo.FindLocations(ctx, repolocations.FindLocationsFilter{
		ScopeID: settlement.ID,
		Depth:   2,
	})
	if err != nil {
		t.Fatalf("FindLocations: %v", err)
	}
	if !containsID(found, tavern.ID) {
		t.Fatalf("find must still see tavern id=%d; children=%#v", tavern.ID, found.Children)
	}
	if !containsID(found, cellar.ID) || !containsID(found, storage.ID) {
		t.Fatalf("find must see cellar=%d and storage=%d; children=%#v", cellar.ID, storage.ID, found.Children)
	}

	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatal("expected player message")
	}
	text := messages[len(messages)-1].Text
	tavernIDStr := strconv.FormatUint(uint64(tavern.ID), 10)
	storageIDStr := strconv.FormatUint(uint64(storage.ID), 10)
	if !strings.Contains(text, tavernIDStr) {
		t.Fatalf("message must mention tavern id=%s, got %q", tavernIDStr, text)
	}
	if !strings.Contains(text, storageIDStr) && !strings.Contains(text, storage.Name) {
		t.Fatalf("message must mention new storage id=%s or name %q, got %q", storageIDStr, storage.Name, text)
	}
	if presentsContradiction(text) {
		t.Fatalf("message contradicts DB state: %q", text)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Tavern after ===\ndesc=%q details=%s\n", gotTavern.Description, gotTavern.Details)
	fmt.Printf("=== Storage room ===\nid=%d name=%q subtype=%v\n", storage.ID, storage.Name, storage.SubtypeID)
	fmt.Printf("=== Player message ===\n%s\n", text)
}

func presentsContradiction(text string) bool {
	lower := strings.ToLower(text)
	// Old keeper as current owner without "former" wording is a contradiction.
	if strings.Contains(text, "Marta") || strings.Contains(text, "Марта") {
		negation := []string{"бывш", "прежн", "раньше", "former", "previous", "used to"}
		ok := false
		for _, h := range negation {
			if strings.Contains(lower, h) {
				ok = true
				break
			}
		}
		if !ok && !strings.Contains(text, "Edwin") && !strings.Contains(text, "Эдвин") {
			return true
		}
	}
	return false
}

func assertDetailsEqual(t *testing.T, wantRaw string, got repositories.JSON) {
	t.Helper()
	var want, have map[string]any
	if err := json.Unmarshal([]byte(wantRaw), &want); err != nil {
		t.Fatalf("unmarshal want: %v", err)
	}
	if err := json.Unmarshal(got, &have); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	wantB, _ := json.Marshal(want)
	haveB, _ := json.Marshal(have)
	if !bytes.Equal(wantB, haveB) {
		t.Fatalf("details mismatch:\nwant %s\ngot  %s", wantRaw, got)
	}
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

func names(locs []repolocations.Location) []string {
	out := make([]string, len(locs))
	for i, loc := range locs {
		out[i] = fmt.Sprintf("%d:%s", loc.ID, loc.Name)
	}
	return out
}

type locationMeta struct {
	Name      string
	ParentID  repositories.ID
	Type      repolocations.LocationType
	SubtypeID repositories.ID
}

type locationFull struct {
	locationMeta
	Description string
	Details     string
}

func snapshotMeta(t *testing.T, id repositories.ID) locationMeta {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	meta := locationMeta{Name: got.Name, Type: got.Type}
	if got.ParentID != nil {
		meta.ParentID = *got.ParentID
	}
	if got.SubtypeID != nil {
		meta.SubtypeID = *got.SubtypeID
	}
	return meta
}

func snapshotFull(t *testing.T, id repositories.ID) locationFull {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	full := locationFull{
		locationMeta: snapshotMeta(t, id),
		Description:  got.Description,
		Details:      string(got.Details),
	}
	return full
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
