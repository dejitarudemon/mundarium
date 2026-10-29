package connection_self_loop_test

import (
	"context"
	"encoding/json"
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
	svclocations "github.com/dejitarudemon/mundarium/internal/services/locations"
	pgstorage "github.com/dejitarudemon/mundarium/internal/storage/postgres"
	"github.com/dejitarudemon/mundarium/internal/tools"
	toolslocations "github.com/dejitarudemon/mundarium/internal/tools/actions/locations"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"gorm.io/gorm"
)

var (
	testConn  *pgstorage.Connection
	testRepo  *repolocations.LocationRepository
	ex        *executor.Executor
	container *postgres.PostgresContainer
)

func TestMain(m *testing.M) {
	// A03: deterministic Executor self-loop connections — Postgres only, no LLM.
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

	generator := func(db *gorm.DB) *repolocations.LocationRepository {
		repo, err := repolocations.NewLocationRepository(db, 5, log)
		if err != nil {
			panic(err)
		}
		return repo
	}

	svc, err := svclocations.NewService(generator, conn, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new locations service: %v\n", err)
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

	createConn, err := toolslocations.NewCreateConnectionTool(
		filepath.Join(root, "definitions", "create_connection.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new create_connection: %v\n", err)
		return
	}
	if createConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "create_connection must be non-idempotent")
		return
	}

	findConn, err := toolslocations.NewFindConnectionsTool(
		filepath.Join(root, "definitions", "find_connections.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new find_connections: %v\n", err)
		return
	}
	if !findConn.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "find_connections must be idempotent")
		return
	}

	updateTo, err := toolslocations.NewUpdateConnectionToTool(
		filepath.Join(root, "definitions", "update_connection_to.json"), svc, log,
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new update_connection_to: %v\n", err)
		return
	}
	if !updateTo.IsIdempotent() {
		fmt.Fprintln(os.Stderr, "update_connection_to must be idempotent")
		return
	}

	for _, tool := range []tools.Tool{createConn, findConn, updateTo} {
		if err := exec.Register(tool, role); err != nil {
			fmt.Fprintf(os.Stderr, "register %s: %v\n", tool.Definition().Name, err)
			return
		}
	}

	testConn = conn
	testRepo = generator(conn.DB())
	ex = exec
	code = m.Run()
}

func TestExecutor_Connection_SelfLoop(t *testing.T) {
	resetDB(t)
	ctx := context.Background()

	room := seedLocation(t, "Anomalous Chamber")
	other := seedLocation(t, "Side Alcove")
	fmt.Printf("=== Seeded locations ===\n  room=%d other=%d\n", room.ID, other.ID)

	t.Run("create_self_loop", func(t *testing.T) {
		args, err := json.Marshal(map[string]any{
			"from_id":       room.ID,
			"to_id":         room.ID,
			"bidirectional": false,
			"description":   "A magical corridor returns the traveler to the same chamber.",
			"state":         map[string]any{"stable": true},
			"details":       map[string]any{"anomaly": "spatial loop"},
			"expires_at":    nil,
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		fmt.Printf("=== create_connection self-loop args ===\n%s\n", args)

		const (
			invokeID = "inv-create-self-loop"
			callID   = "call-create-self-loop"
		)
		raw, err := ex.Execute(ctx, invokeID, tools.Call{
			ID: callID, Name: "create_connection", Role: types.Role_Master, Args: args,
		})
		if err != nil {
			t.Fatalf("create_connection self-loop: %v", err)
		}
		fmt.Printf("=== create_connection result ===\n%s\n", raw)

		var created struct {
			ID     repositories.ID `json:"id"`
			FromID repositories.ID `json:"from_id"`
			ToID   repositories.ID `json:"to_id"`
		}
		if err := json.Unmarshal(raw, &created); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if created.ID == 0 || created.FromID != room.ID || created.ToID != room.ID {
			t.Fatalf("want self-loop on room %d, got %+v", room.ID, created)
		}

		got, err := testRepo.GetConnection(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetConnection: %v", err)
		}
		fmt.Printf("=== Re-read connection ===\n%s\n", mustJSON(got))
		if got.FromID != got.ToID || got.FromID != room.ID {
			t.Fatalf("persisted self-loop mismatch: %+v", got)
		}
		if got.Description == "" {
			t.Fatal("description must be preserved")
		}

		assertJournalCompleted(t, invokeID, callID, "create_connection")

		findArgs, _ := json.Marshal(map[string]any{"location_id": room.ID})
		findRaw, err := ex.Execute(ctx, "inv-find-self-loop", tools.Call{
			ID: "call-find-self-loop", Name: "find_connections", Role: types.Role_Master, Args: findArgs,
		})
		if err != nil {
			t.Fatalf("find_connections: %v", err)
		}
		fmt.Printf("=== find_connections by location_id ===\n%s\n", findRaw)

		var found struct {
			Connections []struct {
				ID     repositories.ID `json:"id"`
				FromID repositories.ID `json:"from_id"`
				ToID   repositories.ID `json:"to_id"`
			} `json:"connections"`
		}
		if err := json.Unmarshal(findRaw, &found); err != nil {
			t.Fatalf("unmarshal find: %v", err)
		}
		if len(found.Connections) != 1 || found.Connections[0].ID != created.ID {
			t.Fatalf("want exactly self-loop id=%d, got %#v", created.ID, found.Connections)
		}
		if found.Connections[0].FromID != found.Connections[0].ToID {
			t.Fatalf("find result must be self-loop, got %#v", found.Connections[0])
		}
		assertNoJournal(t, "inv-find-self-loop", "call-find-self-loop")
	})

	t.Run("update_connection_to_self_loop", func(t *testing.T) {
		// Start as room → other, then retarget to_id to from_id (self-loop).
		conn := &repolocations.Connection{
			FromID:        room.ID,
			ToID:          other.ID,
			Description:   "temporary exit",
			Bidirectional: false,
			State:         repositories.JSON(`{"phase":"open"}`),
			Details:       repositories.JSON(`{"marked":true}`),
		}
		if err := testRepo.CreateConnection(ctx, conn); err != nil {
			t.Fatalf("seed directed connection: %v", err)
		}
		fmt.Printf("=== Seeded directed connection id=%d %d→%d ===\n", conn.ID, conn.FromID, conn.ToID)

		beforeState := string(conn.State)
		beforeDetails := string(conn.Details)
		beforeDesc := conn.Description

		args, err := json.Marshal(map[string]any{
			"id":    conn.ID,
			"to_id": room.ID, // same as from_id
		})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		fmt.Printf("=== update_connection_to self-loop args ===\n%s\n", args)

		raw, err := ex.Execute(ctx, "inv-update-to-self", tools.Call{
			ID: "call-update-to-self", Name: "update_connection_to", Role: types.Role_Master, Args: args,
		})
		if err != nil {
			t.Fatalf("update_connection_to self-loop: %v", err)
		}
		fmt.Printf("=== update_connection_to result ===\n%s\n", raw)

		got, err := testRepo.GetConnection(ctx, conn.ID)
		if err != nil {
			t.Fatalf("GetConnection after patch: %v", err)
		}
		fmt.Printf("=== Connection after update_connection_to ===\n%s\n", mustJSON(got))
		if got.FromID != room.ID || got.ToID != room.ID {
			t.Fatalf("want self-loop room→room, got %d→%d", got.FromID, got.ToID)
		}
		if got.Description != beforeDesc {
			t.Fatalf("description must stay %q, got %q", beforeDesc, got.Description)
		}
		if string(got.State) != beforeState && !jsonEqual(got.State, []byte(beforeState)) {
			t.Fatalf("state must stay unchanged: before=%s after=%s", beforeState, got.State)
		}
		if string(got.Details) != beforeDetails && !jsonEqual(got.Details, []byte(beforeDetails)) {
			t.Fatalf("details must stay unchanged: before=%s after=%s", beforeDetails, got.Details)
		}
		if got.Bidirectional {
			t.Fatal("bidirectional must stay false")
		}
		assertNoJournal(t, "inv-update-to-self", "call-update-to-self")
	})

	// Final readback: both self-loops involving the room must be visible via location_id.
	t.Run("final_find_by_location_id", func(t *testing.T) {
		findArgs, err := json.Marshal(map[string]any{"location_id": room.ID})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		fmt.Printf("=== final find_connections location_id=%d args ===\n%s\n", room.ID, findArgs)

		findRaw, err := ex.Execute(ctx, "inv-find-final-self-loops", tools.Call{
			ID: "call-find-final-self-loops", Name: "find_connections", Role: types.Role_Master, Args: findArgs,
		})
		if err != nil {
			t.Fatalf("find_connections: %v", err)
		}
		fmt.Printf("=== final find_connections result ===\n%s\n", findRaw)

		var found struct {
			Connections []struct {
				ID     repositories.ID `json:"id"`
				FromID repositories.ID `json:"from_id"`
				ToID   repositories.ID `json:"to_id"`
			} `json:"connections"`
		}
		if err := json.Unmarshal(findRaw, &found); err != nil {
			t.Fatalf("unmarshal find: %v", err)
		}
		if len(found.Connections) != 2 {
			t.Fatalf("want 2 self-loops for location_id=%d, got %#v", room.ID, found.Connections)
		}
		for _, c := range found.Connections {
			if c.FromID != room.ID || c.ToID != room.ID {
				t.Fatalf("want self-loop on room %d, got id=%d %d→%d", room.ID, c.ID, c.FromID, c.ToID)
			}
		}
		assertNoJournal(t, "inv-find-final-self-loops", "call-find-final-self-loops")
	})
}

func seedLocation(t *testing.T, name string) *repolocations.Location {
	t.Helper()
	loc, err := testRepo.CreateLocation(context.Background(), &repolocations.Location{
		Type:        repolocations.LocationType_ROOM,
		Name:        name,
		Description: name,
		Details:     repositories.JSON(`{}`),
	})
	if err != nil {
		t.Fatalf("seed location %q: %v", name, err)
	}
	return loc
}

func assertJournalCompleted(t *testing.T, invokeID, callID, tool string) {
	t.Helper()
	var req journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND id = ?", invokeID, callID).
		First(&req).Error; err != nil {
		t.Fatalf("load journal %s/%s: %v", invokeID, callID, err)
	}
	fmt.Printf("=== Journal ===\nid=%s group=%s tool=%s status=%s\n", req.ID, req.GroupID, req.Tool, req.Status)
	if req.Tool != tool {
		t.Fatalf("journal tool: want %s, got %s", tool, req.Tool)
	}
	if req.Status != journal.Status_Completed {
		t.Fatalf("journal status: want %s, got %s err=%v", journal.Status_Completed, req.Status, req.Error)
	}
}

func assertNoJournal(t *testing.T, invokeID, callID string) {
	t.Helper()
	var n int64
	if err := testConn.DB().
		Table("execution_journal_requests").
		Where("group_id = ? AND id = ?", invokeID, callID).
		Count(&n).Error; err != nil {
		t.Fatalf("count journal: %v", err)
	}
	if n != 0 {
		t.Fatalf("idempotent tool must not journal, got %d rows for %s/%s", n, invokeID, callID)
	}
}

func jsonEqual(a, b []byte) bool {
	var left, right any
	if err := json.Unmarshal(a, &left); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &right); err != nil {
		return false
	}
	la, _ := json.Marshal(left)
	ra, _ := json.Marshal(right)
	return string(la) == string(ra)
}

func mustJSON(v any) string {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
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
