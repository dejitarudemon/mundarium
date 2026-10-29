package dices_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	"github.com/dejitarudemon/mundarium/internal/providers"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/dejitarudemon/mundarium/tests/testutil"
	"github.com/google/uuid"
)

var (
	toolSeqMu sync.Mutex
	toolSeq   []toolCallRec
)

type toolCallRec struct {
	Name   string
	Args   json.RawMessage
	Result json.RawMessage
	Err    error
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
	name := r.inner.Definition().Name
	argsCopy := append(json.RawMessage(nil), args...)

	out, err := r.inner.Execute(ctx, args)

	toolSeqMu.Lock()
	rec := toolCallRec{Name: name, Args: argsCopy, Err: err}
	if out != nil {
		rec.Result = append(json.RawMessage(nil), out...)
	}
	toolSeq = append(toolSeq, rec)
	toolSeqMu.Unlock()
	return out, err
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

func toolNames(seq []toolCallRec) []string {
	out := make([]string, len(seq))
	for i, rec := range seq {
		out[i] = rec.Name
	}
	return out
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

func firstTool(seq []toolCallRec, name string) toolCallRec {
	for _, rec := range seq {
		if rec.Name == name {
			return rec
		}
	}
	return toolCallRec{}
}

func resetDB(t *testing.T) {
	t.Helper()
	if err := testConn.DB().Exec(`
		TRUNCATE TABLE messages, execution_journal_requests
		RESTART IDENTITY CASCADE
	`).Error; err != nil {
		t.Fatalf("truncate tables: %v", err)
	}
	resetToolSeq()
}

type invokeReport struct {
	ID        string
	Elapsed   time.Duration
	ToolCalls []toolCallRec
	Journal   []journal.Request
}

func invoke(t *testing.T, scenario, userMessage string) invokeReport {
	t.Helper()
	timeout := time.Duration(testutil.EnvInt(testutil.EnvTimeoutSec, 600)) * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	invokeID := uuid.NewString()
	started := time.Now()
	fmt.Printf("\n=== Invoke %s ===\nmodel=%s url=%s max_iters=%d\ninvoke_id=%s\nuser=%s\n",
		scenario, llmModel, llmURL, llmMaxIter, invokeID, userMessage)

	err := provider.Invoke(ctx, invokeID, providers.Request{
		Args: json.RawMessage(userMessage),
	})
	elapsed := time.Since(started)
	fmt.Printf("=== Invoke done in %s err=%v ===\n", elapsed, err)

	seq := snapshotToolSeq()
	fmt.Printf("=== Tool calls (%d) ===\n", len(seq))
	for i, rec := range seq {
		fmt.Printf("%d. %s args=%s result=%s err=%v\n",
			i+1, rec.Name, compactJSON(rec.Args), compactJSON(rec.Result), rec.Err)
	}

	var journaled []journal.Request
	if jerr := testConn.DB().Where("group_id = ?", invokeID).Order("created_at asc").Find(&journaled).Error; jerr != nil {
		t.Fatalf("load journal: %v", jerr)
	}
	fmt.Printf("=== Journal (%d) ===\n", len(journaled))
	for _, req := range journaled {
		fmt.Printf("tool=%s status=%s err=%v\n", req.Tool, req.Status, req.Error)
	}

	if msg, ok := lastPlayerMessage(t, invokeID); ok {
		fmt.Printf("=== Player message ===\n%s\n", msg)
	}

	if err != nil {
		t.Fatalf("scenario=%s Invoke: %v", scenario, err)
	}
	return invokeReport{ID: invokeID, Elapsed: elapsed, ToolCalls: seq, Journal: journaled}
}

func requireTool(t *testing.T, seq []toolCallRec, name string) {
	t.Helper()
	if countTool(seq, name) < 1 {
		t.Fatalf("ToolSelectionError: expected %s, got sequence %#v", name, toolNames(seq))
	}
}

func requireNoTool(t *testing.T, seq []toolCallRec, name string) {
	t.Helper()
	if countTool(seq, name) != 0 {
		t.Fatalf("ToolSelectionError: did not expect %s, got %#v", name, toolNames(seq))
	}
}

func requireRollCompleted(t *testing.T, report invokeReport) {
	t.Helper()
	var found bool
	for _, req := range report.Journal {
		if req.Tool != "roll_dice" {
			continue
		}
		found = true
		if req.Status != journal.Status_Completed {
			t.Fatalf("ExecutionError: roll_dice status=%s err=%v", req.Status, req.Error)
		}
	}
	if !found {
		t.Fatal("ExecutionError: no roll_dice journal row")
	}
}

type rollArgs struct {
	Groups []repodices.Die `json:"dieces_group"`
}

type rollResult struct {
	Total int `json:"total"`
	Dice  []struct {
		Sides uint8  `json:"sides"`
		Rolls []uint `json:"rolls"`
	} `json:"dice"`
}

func parseRollArgs(t *testing.T, raw json.RawMessage) rollArgs {
	t.Helper()
	var args rollArgs
	if len(raw) == 0 {
		t.Fatal("empty roll_dice args")
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatalf("unmarshal roll_dice args %s: %v", raw, err)
	}
	if len(args.Groups) == 0 {
		t.Fatalf("roll_dice args missing dieces_group: %s", raw)
	}
	return args
}

func parseRollResult(t *testing.T, raw json.RawMessage) rollResult {
	t.Helper()
	var out rollResult
	if len(raw) == 0 {
		t.Fatal("empty roll_dice result")
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal roll_dice result %s: %v", raw, err)
	}
	return out
}

func assertPlainFacesInRange(t *testing.T, rolls []uint, sides uint8) {
	t.Helper()
	for i, face := range rolls {
		if face < 1 || face > uint(sides) {
			t.Fatalf("face[%d]=%d: want in [1,%d]", i, face, sides)
		}
	}
}

func sumRolls(result rollResult) int {
	sum := 0
	for _, g := range result.Dice {
		for _, face := range g.Rolls {
			sum += int(face)
		}
	}
	return sum
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

func requireMessageMentions(t *testing.T, invokeID string, values ...int) {
	t.Helper()
	msg, ok := lastPlayerMessage(t, invokeID)
	if !ok {
		t.Fatal("ResultUseError: expected send_message with player-facing text")
	}
	lower := strings.ToLower(msg)
	for _, v := range values {
		needle := strconv.Itoa(v)
		if !strings.Contains(msg, needle) && !strings.Contains(lower, needle) {
			t.Fatalf("ResultUseError: player message %q does not mention %d", msg, v)
		}
	}
}

func compactJSON(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
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

func findGroupBySides(args rollArgs, sides uint8) (repodices.Die, bool) {
	for _, g := range args.Groups {
		if g.Sides == sides {
			return g, true
		}
	}
	return repodices.Die{}, false
}
