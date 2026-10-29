package update_location_description_only_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
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
	tavernName      = "The Copper Fox"
	oldDescription  = "A quiet copper-signed tavern by the market square."
	newDescription  = "Smoke curls from the chimney; the Copper Fox now bustles with traveling merchants."
	seededDetails   = `{"keeper":"Marta","rumor":"A map is hidden beneath the hearth","rooms":4}`
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
		fmt.Fprintf(os.Stderr, "skip update_location_description_only Invoke tests: %v\n", reason)
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
	updateTool, err := toolslocations.NewUpdateLocationDescriptionTool(
		filepath.Join(root, "definitions", "update_location_description.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_location_description tool: %v\n", err)
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
	if err := exec.Register(updateTool, role); err != nil {
		fmt.Fprintf(os.Stderr, "register update_location_description: %v\n", err)
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

func TestInvoke_UpdateLocationDescription_Only(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

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

	settlement, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "Ashford",
		Description: "market town",
		Details:     repositories.JSON(`{"size":"medium"}`),
	})
	if err != nil {
		t.Fatalf("seed settlement: %v", err)
	}

	tavern, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    new(settlement.ID),
		Type:        repolocations.LocationType_STRUCTURE,
		SubtypeID:   new(subtype.ID),
		Name:        tavernName,
		Description: oldDescription,
		Details:     repositories.JSON(seededDetails),
	})
	if err != nil {
		t.Fatalf("seed tavern: %v", err)
	}

	before := snapshotExceptDescription(t, tavern.ID)

	userMessage := fmt.Sprintf(
		`У локации %q (id=%d) полностью замени description через update_location_description на точный текст (без дополнения старого):
%q
Details и остальные поля не меняй. Затем одним send_message подтверди id и новое описание. Другие локации не создавай.`,
		tavernName, tavern.ID, newDescription,
	)

	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 180)) * time.Second
	invokeCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	fmt.Printf("=== Seed ===\ntavern=%d old_description=%q\n", tavern.ID, oldDescription)
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
		case "update_location_description", "find_locations":
			t.Fatalf("%s is idempotent and must not be journaled, got %#v", req.Tool, req)
		case "create_location", "create_subtype", "update_location_details":
			t.Fatalf("%s must not be called, got journal %#v", req.Tool, req)
		case "send_message":
			sendRows++
			if req.Status != journal.Status_Completed {
				t.Fatalf("send_message %s: want %s, got %s", req.ID, journal.Status_Completed, req.Status)
			}
		}
	}
	if sendRows == 0 {
		t.Fatal("expected at least one send_message journal row (update_location_description itself is not journaled)")
	}

	assertDescriptionReplaced(t, tavern.ID, before)

	afterInvoke := dumpLocation(t, tavern.ID)

	// Repeat the same update deterministically — state must stay identical.
	args, err := json.Marshal(map[string]any{
		"id":          tavern.ID,
		"description": newDescription,
	})
	if err != nil {
		t.Fatalf("marshal repeat args: %v", err)
	}
	raw, err := ex.Execute(ctx, uuid.NewString(), tools.Call{
		ID:   "update-description-again",
		Name: "update_location_description",
		Role: types.Role_Master,
		Args: args,
	})
	if err != nil {
		t.Fatalf("Executor.Execute update_location_description: %v", err)
	}
	var out struct {
		ID          repositories.ID `json:"id"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal result: %v\nraw=%s", err, raw)
	}
	if out.ID != tavern.ID || out.Description != newDescription {
		t.Fatalf("repeat result: want id=%d description=%q, got id=%d description=%q",
			tavern.ID, newDescription, out.ID, out.Description)
	}

	assertDescriptionReplaced(t, tavern.ID, before)
	afterAgain := dumpLocation(t, tavern.ID)
	if afterAgain != afterInvoke {
		t.Fatalf("repeat must not change state:\nbefore=%s\nafter=%s", afterInvoke, afterAgain)
	}

	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s\n", req.Tool, req.Status)
	}
	fmt.Printf("=== Location after ===\n%s\n", afterAgain)
}

func assertDescriptionReplaced(t *testing.T, id repositories.ID, before locationExceptDescription) {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	if got.Description != newDescription {
		t.Fatalf("description: want exact %q, got %q", newDescription, got.Description)
	}
	if strings.Contains(got.Description, oldDescription) {
		t.Fatalf("old description must be replaced, not appended: %q", got.Description)
	}
	var wantDetails, haveDetails map[string]any
	if err := json.Unmarshal([]byte(seededDetails), &wantDetails); err != nil {
		t.Fatalf("unmarshal want details: %v", err)
	}
	if err := json.Unmarshal(got.Details, &haveDetails); err != nil {
		t.Fatalf("unmarshal got details: %v", err)
	}
	wantRaw, _ := json.Marshal(wantDetails)
	haveRaw, _ := json.Marshal(haveDetails)
	if !bytes.Equal(wantRaw, haveRaw) {
		t.Fatalf("details must be unchanged:\nwant %s\ngot  %s", seededDetails, got.Details)
	}

	after := snapshotExceptDescription(t, id)
	if after != before {
		t.Fatalf("non-description fields must not change:\nbefore=%+v\nafter=%+v", before, after)
	}
}

type locationExceptDescription struct {
	Name      string
	Details   string
	ParentID  repositories.ID
	Type      repolocations.LocationType
	SubtypeID repositories.ID
}

func snapshotExceptDescription(t *testing.T, id repositories.ID) locationExceptDescription {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	snap := locationExceptDescription{
		Name:    got.Name,
		Details: string(got.Details),
		Type:    got.Type,
	}
	if got.ParentID != nil {
		snap.ParentID = *got.ParentID
	}
	if got.SubtypeID != nil {
		snap.SubtypeID = *got.SubtypeID
	}
	return snap
}

func dumpLocation(t *testing.T, id repositories.ID) string {
	t.Helper()
	got, err := testRepo.GetLocation(context.Background(), id)
	if err != nil {
		t.Fatalf("GetLocation(%d): %v", id, err)
	}
	view := struct {
		ID          repositories.ID   `json:"id"`
		ParentID    *repositories.ID  `json:"parent_id"`
		Type        string            `json:"type"`
		SubtypeID   *repositories.ID  `json:"subtype_id"`
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Details     repositories.JSON `json:"details"`
	}{
		ID:          got.ID,
		ParentID:    got.ParentID,
		Type:        got.Type.String(),
		SubtypeID:   got.SubtypeID,
		Name:        got.Name,
		Description: got.Description,
		Details:     got.Details,
	}
	raw, err := json.Marshal(view)
	if err != nil {
		t.Fatalf("marshal dump: %v", err)
	}
	return string(raw)
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
