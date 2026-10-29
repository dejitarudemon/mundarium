package dungeon_master_goblin_crit_test

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/execution"
	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
	repodices "github.com/dejitarudemon/mundarium/internal/repositories/dices"
	repomessages "github.com/dejitarudemon/mundarium/internal/repositories/messages"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/dejitarudemon/mundarium/internal/tools/definition"
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

type recordingTool struct{ inner tools.Tool }

func recordTool(inner tools.Tool) tools.Tool { return &recordingTool{inner: inner} }

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

func toolsNamed(seq []toolCallRec, name string) []toolCallRec {
	var out []toolCallRec
	for _, rec := range seq {
		if rec.Name == name {
			out = append(out, rec)
		}
	}
	return out
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

func parseRollArgs(raw json.RawMessage) (rollArgs, error) {
	var args rollArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return rollArgs{}, err
	}
	return args, nil
}

func parseRollResult(raw json.RawMessage) (rollResult, error) {
	var out rollResult
	if err := json.Unmarshal(raw, &out); err != nil {
		return rollResult{}, err
	}
	return out, nil
}

func sumFaces(r rollResult) int {
	sum := 0
	for _, g := range r.Dice {
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

func messageMentionsInt(text string, n int) bool {
	return strings.Contains(text, strconv.Itoa(n))
}

func lookLikeHit(text string) bool {
	lower := strings.ToLower(text)
	hitWords := []string{
		"попал", "попала", "попало", "попадание", "попадае",
		"ранит", "ранен", "получаешь урон", "наносит урон", "критическ",
		"впива", "вонзается тебе", "вонзилась тебе", "в плечо", "в грудь",
		"урон:", "колющего", "крит", "двойной",
	}
	hit := false
	for _, w := range hitWords {
		if strings.Contains(lower, w) {
			hit = true
			break
		}
	}
	if !hit {
		return false
	}
	// Explicit miss wording overrides atmospheric hit cues.
	if lookLikeMiss(text) && !strings.Contains(lower, "попадание") && !strings.Contains(lower, "урон:") {
		return false
	}
	return true
}

func lookLikeMiss(text string) bool {
	lower := strings.ToLower(text)
	missWords := []string{
		"промах", "мимо твоего", "мимо тебя", "пролетает мимо", "проходит мимо",
		"не попал", "не попала", "не попало",
		"дюймах от", "в стену позади", "в каменную стену", "за твоей спиной", "позади тебя",
		"не задел", "не задела", "не задело",
	}
	for _, w := range missWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

func failf(failures *[]string, format string, args ...any) {
	*failures = append(*failures, fmt.Sprintf(format, args...))
}
