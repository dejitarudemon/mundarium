package executor_non_idempotent_replay_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	pgjournal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution/implemented/postgres"
	"github.com/dejitarudemon/mundarium/internal/libs/logger/implemented"
	"github.com/dejitarudemon/mundarium/internal/libs/types"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	svcmessages "github.com/dejitarudemon/mundarium/internal/services/messages"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	toolsmessages "github.com/dejitarudemon/mundarium/internal/tools/actions/messages"
	toolerrs "github.com/dejitarudemon/mundarium/internal/tools/errs"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"golang.org/x/crypto/blake2b"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	jnl       journal.Journal
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// Deterministic Executor journal replay: Postgres only, no LLM.
	_ = testutil.LoadDotEnvFromRepo()

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

	createLoc, err := toolslocations.NewCreateLocationTool(
		filepath.Join(root, "definitions", "create_location.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_location: %v\n", err)
		return
	}
	createSubtype, err := toolslocations.NewCreateSubtypeTool(
		filepath.Join(root, "definitions", "create_subtype.json"),
		locSvc,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_subtype: %v\n", err)
		return
	}
	sendTool, err := toolsmessages.NewCreateMessageTool(
		filepath.Join(root, "definitions", "send_message.json"),
		msgSvc,
		nil,
		log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new send_message: %v\n", err)
		return
	}

	for _, tool := range []tools.Tool{createLoc, createSubtype, sendTool} {
		if tool.IsIdempotent() {
			fmt.Fprintf(os.Stderr, "%s must be non-idempotent\n", tool.Definition().Name)
			return
		}
		if err := exec.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", tool.Definition().Name, err)
			return
		}
	}

	testConn = conn
	testRepo = locGenerator(conn.DB())
	ex = exec
	jnl = j
	code = m.Run()
}

func TestExecutor_NonIdempotentCallReplay(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	parent, err := testRepo.CreateLocation(ctx, &repolocations.Location{
		ParentID:    nil,
		Type:        repolocations.LocationType_SETTLEMENT,
		Name:        "Ashford",
		Description: "seed parent",
		Details:     repositories.JSON(`{"seed":true}`),
	})
	if err != nil {
		t.Fatalf("seed parent: %v", err)
	}

	t.Run("create_location_replay", func(t *testing.T) {
		const (
			invokeID = "inv-create-loc"
			callID   = "call-create-loc"
		)
		args, err := json.Marshal(map[string]any{
			"name":        "The Copper Fox",
			"type":        "structure",
			"parent_id":   parent.ID,
			"description": "tavern by the square",
			"details":     map[string]any{"keeper": "Marta"},
			"subtype":     nil,
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		call := tools.Call{ID: callID, Name: "create_location", Role: types.Role_Master, Args: args}

		first, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("first create_location: %v", err)
		}
		assertJournalCompleted(t, invokeID, callID, "create_location")

		var locsBefore int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsBefore).Error; err != nil {
			t.Fatalf("count: %v", err)
		}

		second, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("replay create_location: %v", err)
		}
		assertJSONEqual(t, first, second)

		var locsAfter int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsAfter).Error; err != nil {
			t.Fatalf("count after: %v", err)
		}
		if locsAfter != locsBefore {
			t.Fatalf("replay must not create another location: before=%d after=%d", locsBefore, locsAfter)
		}
		assertJournalCompleted(t, invokeID, callID, "create_location")
		assertSingleJournalRow(t, invokeID, callID)
	})

	t.Run("create_subtype_replay", func(t *testing.T) {
		const (
			invokeID = "inv-create-subtype"
			callID   = "call-create-subtype"
		)
		args, err := json.Marshal(map[string]any{
			"key":           "tavern",
			"type":          "structure",
			"defining_tags": []string{"rest"},
			"expected_tags": []string{"village"},
			"description":   "inn",
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		call := tools.Call{ID: callID, Name: "create_subtype", Role: types.Role_Master, Args: args}

		first, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("first create_subtype: %v", err)
		}
		assertJournalCompleted(t, invokeID, callID, "create_subtype")

		var countBefore int64
		if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&countBefore).Error; err != nil {
			t.Fatalf("count subtypes: %v", err)
		}

		second, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("replay create_subtype: %v", err)
		}
		assertJSONEqual(t, first, second)

		var countAfter int64
		if err := testConn.DB().Model(&repolocations.LocationSubtype{}).Count(&countAfter).Error; err != nil {
			t.Fatalf("count subtypes after: %v", err)
		}
		if countAfter != countBefore {
			t.Fatalf("replay must not create another subtype: before=%d after=%d", countBefore, countAfter)
		}
		assertJournalCompleted(t, invokeID, callID, "create_subtype")
		assertSingleJournalRow(t, invokeID, callID)
	})

	t.Run("send_message_replay", func(t *testing.T) {
		const (
			invokeID = "inv-send-message"
			callID   = "call-send-message"
		)
		args := json.RawMessage(`{"text":"The tavern door creaks open."}`)
		call := tools.Call{ID: callID, Name: "send_message", Role: types.Role_Master, Args: args}

		first, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("first send_message: %v", err)
		}
		assertJournalCompleted(t, invokeID, callID, "send_message")

		var msgBefore int64
		if err := testConn.DB().Model(&repomessages.Message{}).
			Where("invoke_id = ?", invokeID).
			Count(&msgBefore).Error; err != nil {
			t.Fatalf("count messages: %v", err)
		}
		if msgBefore != 1 {
			t.Fatalf("want 1 message after first send, got %d", msgBefore)
		}

		second, err := ex.Execute(ctx, invokeID, call)
		if err != nil {
			t.Fatalf("replay send_message: %v", err)
		}
		assertJSONEqual(t, first, second)

		var msgAfter int64
		if err := testConn.DB().Model(&repomessages.Message{}).
			Where("invoke_id = ?", invokeID).
			Count(&msgAfter).Error; err != nil {
			t.Fatalf("count messages after: %v", err)
		}
		if msgAfter != 1 {
			t.Fatalf("replay must not create another message: before=%d after=%d", msgBefore, msgAfter)
		}
		assertJournalCompleted(t, invokeID, callID, "send_message")
		assertSingleJournalRow(t, invokeID, callID)
	})

	t.Run("running_not_treated_as_completed", func(t *testing.T) {
		const (
			invokeID = "inv-running"
			callID   = "call-running"
		)
		args := json.RawMessage(fmt.Sprintf(
			`{"name":"ShouldNotExist","type":"structure","parent_id":%d,"description":"x","details":{"a":1},"subtype":null}`,
			parent.ID,
		))
		if _, _, err := jnl.Register(ctx, journal.ID(callID), journal.ID(invokeID), "create_location", hashArgs(args)); err != nil {
			t.Fatalf("seed Register running: %v", err)
		}

		var locsBefore int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsBefore).Error; err != nil {
			t.Fatalf("count: %v", err)
		}

		_, err := ex.Execute(ctx, invokeID, tools.Call{
			ID: callID, Name: "create_location", Role: types.Role_Master, Args: args,
		})
		if _, ok := errors.AsType[toolerrs.RequestStillRunning](err); !ok {
			t.Fatalf("want RequestStillRunning, got %v", err)
		}

		var locsAfter int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsAfter).Error; err != nil {
			t.Fatalf("count after: %v", err)
		}
		if locsAfter != locsBefore {
			t.Fatalf("running request must not execute create: before=%d after=%d", locsBefore, locsAfter)
		}

		req := loadJournal(t, invokeID, callID)
		if req.Status != journal.Status_Running {
			t.Fatalf("journal must stay running, got %s", req.Status)
		}
	})

	t.Run("failed_not_treated_as_completed", func(t *testing.T) {
		const (
			invokeID = "inv-failed"
			callID   = "call-failed"
		)
		// Fail create via missing subtype FK, then replay same identity.
		args := json.RawMessage(fmt.Sprintf(
			`{"name":"BadSubtype","type":"structure","parent_id":%d,"description":"x","details":{"a":1},"subtype":999999}`,
			parent.ID,
		))
		call := tools.Call{ID: callID, Name: "create_location", Role: types.Role_Master, Args: args}

		firstRaw, err := ex.Execute(ctx, invokeID, call)
		if err == nil {
			t.Fatalf("first create with bad subtype must fail, got %s", firstRaw)
		}
		req := loadJournal(t, invokeID, callID)
		if req.Status != journal.Status_Failed {
			t.Fatalf("journal want failed, got %s", req.Status)
		}

		var locsBefore int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsBefore).Error; err != nil {
			t.Fatalf("count: %v", err)
		}

		secondRaw, err := ex.Execute(ctx, invokeID, call)
		if err == nil {
			t.Fatalf("replay of failed call must not succeed, got %s", secondRaw)
		}
		if len(secondRaw) != 0 {
			t.Fatalf("failed replay must not return success payload, got %s", secondRaw)
		}

		var locsAfter int64
		if err := testConn.DB().Model(&repolocations.Location{}).Count(&locsAfter).Error; err != nil {
			t.Fatalf("count after: %v", err)
		}
		if locsAfter != locsBefore {
			t.Fatalf("failed replay must not mutate locations: before=%d after=%d", locsBefore, locsAfter)
		}

		req2 := loadJournal(t, invokeID, callID)
		if req2.Status != journal.Status_Failed {
			t.Fatalf("journal must remain failed, got %s", req2.Status)
		}
		assertSingleJournalRow(t, invokeID, callID)
	})
}

func assertJournalCompleted(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	req := loadJournal(t, invokeID, callID)
	if req.Tool != tool {
		t.Fatalf("journal tool: want %s, got %s", tool, req.Tool)
	}
	if req.Status != journal.Status_Completed {
		t.Fatalf("journal status: want %s, got %s err=%v", journal.Status_Completed, req.Status, req.Error)
	}
	if req.Result.Data == nil {
		t.Fatal("completed journal must store result data")
	}
}

func assertSingleJournalRow(t *testing.T, invokeID, callID string) {
	t.Helper()
	var n int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ? AND id = ?", invokeID, callID).
		Count(&n).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if n != 1 {
		t.Fatalf("want exactly 1 journal row for %s/%s, got %d", invokeID, callID, n)
	}
}

func loadJournal(t *testing.T, invokeID, callID string) journal.Request {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal %s/%s: %v", invokeID, callID, err)
	}
	return req
}

func assertJSONEqual(t *testing.T, a, b json.RawMessage) {
	t.Helper()
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		t.Fatalf("unmarshal first: %v\n%s", err, a)
	}
	if err := json.Unmarshal(b, &right); err != nil {
		t.Fatalf("unmarshal second: %v\n%s", err, b)
	}
	leftRaw, _ := json.Marshal(left)
	rightRaw, _ := json.Marshal(right)
	if string(leftRaw) != string(rightRaw) {
		t.Fatalf("replay payload mismatch:\nfirst=%s\nsecond=%s", a, b)
	}
}

func hashArgs(args json.RawMessage) []byte {
	sum := blake2b.Sum256(args)
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
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
