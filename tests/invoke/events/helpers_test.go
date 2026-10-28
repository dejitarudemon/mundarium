package events_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/providers"
	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

var (
	toolSeqMu sync.Mutex
	toolSeq   []toolCallRec
)

type toolCallRec struct {
	Name string
	Args json.RawMessage
}

type recordingTool struct {
	inner tools.Tool
}

func recordTool(inner tools.Tool) tools.Tool {
	return &recordingTool{inner: inner}
}

func (r *recordingTool) Definition() definition.Definition { return r.inner.Definition() }
func (r *recordingTool) IsIdempotent() bool                { return r.inner.IsIdempotent() }

func (r *recordingTool) Execute(ctx execution.ExecutionContext[journal.Result], args json.RawMessage) (json.RawMessage, error) {
	toolSeqMu.Lock()
	toolSeq = append(toolSeq, toolCallRec{
		Name: r.inner.Definition().Name,
		Args: append(json.RawMessage(nil), args...),
	})
	toolSeqMu.Unlock()
	return r.inner.Execute(ctx, args)
}

func resetToolSeq() {
	toolSeqMu.Lock()
	toolSeq = nil
	toolSeqMu.Unlock()
}

func snapshotToolSeq() []toolCallRec {
	toolSeqMu.Lock()
	defer toolSeqMu.Unlock()
	out := make([]toolCallRec, len(toolSeq))
	copy(out, toolSeq)
	return out
}

func countTool(seq []toolCallRec, name string) int {
	n := 0
	for _, rec := range seq {
		if rec.Name == name {
			n++
		}
	}
	return n
}

func toolsNamed(seq []toolCallRec, name string) []toolCallRec {
	var out []toolCallRec
	for _, rec := range seq {
		if rec.Name == name {
			out = append(out, rec)
		}
	}
	return out
}

func firstToolArgs(seq []toolCallRec, name string) json.RawMessage {
	for _, rec := range seq {
		if rec.Name == name {
			return rec.Args
		}
	}
	return nil
}

func lastToolArgs(seq []toolCallRec, name string) json.RawMessage {
	var last json.RawMessage
	for _, rec := range seq {
		if rec.Name == name {
			last = rec.Args
		}
	}
	return last
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, event_causals, event_entities, events, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	resetToolSeq()
}

func invoke(t *testing.T, userMessage string) string {
	t.Helper()
	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 600)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	started := time.Now()
	fmt.Printf("\n=== Invoke ===\nmodel=%s url=%s max_iters=%d\ninvoke_id=%s\nuser=%s\n",
		llmModel, llmURL, llmMaxIter, invokeID, userMessage)

	err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	})
	elapsed := time.Since(started)
	fmt.Printf("=== Invoke done in %s err=%v ===\n", elapsed, err)

	seq := snapshotToolSeq()
	fmt.Printf("=== Tool calls (%d) ===\n", len(seq))
	for i, rec := range seq {
		fmt.Printf("%d. %s %s\n", i+1, rec.Name, compactJSON(rec.Args))
	}

	var journaled []journal.Request
	if jerr := testConn.DB().Where("group_id = ?", invokeID).Order("created_at asc").Find(&journaled).Error; jerr != nil {
		t.Fatalf("load journal: %v", jerr)
	}
	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s err=%v\n", req.Tool, req.Status, req.Error)
	}

	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	return invokeID
}

func seedEvent(t *testing.T, summary string, importance int8, ticks repositories.Ticks, opts ...func(*repoevents.Event)) *repoevents.Event {
	t.Helper()
	ev := &repoevents.Event{
		Importance:        importance,
		CreatedAtGameTime: ticks,
		Summary:           summary,
		Tags:              pq.StringArray{},
		Details:           repositories.JSON(`{}`),
	}
	for _, opt := range opts {
		opt(ev)
	}
	got, err := testRepo.CreateEvent(context.Background(), ev)
	if err != nil {
		t.Fatalf("seed event %q: %v", summary, err)
	}
	return got
}

func withTags(tags ...string) func(*repoevents.Event) {
	return func(e *repoevents.Event) { e.Tags = pq.StringArray(tags) }
}

func withLocation(id repositories.ID) func(*repoevents.Event) {
	return func(e *repoevents.Event) { e.LocationID = &id }
}

func withDetails(raw string) func(*repoevents.Event) {
	return func(e *repoevents.Event) { e.Details = repositories.JSON(raw) }
}

func getEvent(t *testing.T, id repoevents.ID) *repoevents.Event {
	t.Helper()
	ev, err := testRepo.GetEvent(context.Background(), id)
	if err != nil {
		t.Fatalf("GetEvent %s: %v", id, err)
	}
	return ev
}

func countEvents(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.Event{}).Count(&n).Error; err != nil {
		t.Fatalf("count events: %v", err)
	}
	return n
}

func countCausals(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.EventCausal{}).Count(&n).Error; err != nil {
		t.Fatalf("count causals: %v", err)
	}
	return n
}

func countEntities(t *testing.T) int64 {
	t.Helper()
	var n int64
	if err := testConn.DB().Model(&repoevents.EventEntity{}).Count(&n).Error; err != nil {
		t.Fatalf("count entities: %v", err)
	}
	return n
}

func listEvents(t *testing.T) []repoevents.Event {
	t.Helper()
	var all []repoevents.Event
	if err := testConn.DB().Order("created_at_game_time, id").Find(&all).Error; err != nil {
		t.Fatalf("list events: %v", err)
	}
	return all
}

func listCausals(t *testing.T) []repoevents.EventCausal {
	t.Helper()
	var all []repoevents.EventCausal
	if err := testConn.DB().Order("from_id, to_id").Find(&all).Error; err != nil {
		t.Fatalf("list causals: %v", err)
	}
	return all
}

func newEntityID() entities.ID { return entities.New() }

func requireTool(t *testing.T, seq []toolCallRec, name string) {
	t.Helper()
	if countTool(seq, name) < 1 {
		t.Fatalf("ToolSelectionError: expected %s, got sequence %#v", name, toolNames(seq))
	}
}

func toolNames(seq []toolCallRec) []string {
	out := make([]string, len(seq))
	for i, rec := range seq {
		out[i] = rec.Name
	}
	return out
}

func unmarshalArgs(t *testing.T, raw json.RawMessage, dst any) {
	t.Helper()
	if len(raw) == 0 {
		t.Fatal("empty tool args")
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatalf("unmarshal args %s: %v", raw, err)
	}
}

func compactJSON(raw json.RawMessage) string {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return string(raw)
	}
	return string(b)
}

func lastPlayerMessage(t *testing.T, invokeID string) (string, bool) {
	t.Helper()
	var messages []repomessages.Message
	if err := testConn.DB().
		Where("invoke_id = ?", invokeID).
		Order("seq_num asc").
		Find(&messages).Error; err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) == 0 {
		return "", false
	}
	return messages[len(messages)-1].Text, true
}

func journalHasFailed(t *testing.T, invokeID, tool string) bool {
	t.Helper()
	var reqs []journal.Request
	if err := testConn.DB().
		Where("group_id = ? AND tool = ?", invokeID, tool).
		Find(&reqs).Error; err != nil {
		t.Fatalf("journal: %v", err)
	}
	for _, req := range reqs {
		if req.Status == journal.Status_Failed {
			return true
		}
	}
	return false
}

func findEventBySummarySubstr(t *testing.T, substr string) *repoevents.Event {
	t.Helper()
	needle := strings.ToLower(substr)
	all := listEvents(t)
	for i := range all {
		if strings.Contains(strings.ToLower(all[i].Summary), needle) {
			return &all[i]
		}
	}
	return nil
}
