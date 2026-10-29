package events_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
)

func TestInvoke_EventEntity_AddAndKnowledge(t *testing.T) {
	resetDB(t)

	ev := seedEvent(t, "Барон приказал казнить мельника", 70, 10, withTags("politics"))
	entityID := newEntityID()

	t.Run("create_witness", func(t *testing.T) {
		resetToolSeq()
		invoke(t, fmt.Sprintf(
			`Добавь персонажа %s в участники события о казни мельника (event_id=%s). Его роль — свидетель (witness).`,
			entityID.String(), ev.ID.String(),
		))
		seq := snapshotToolSeq()
		requireTool(t, seq, "create_event_entity")
		// May find the event first.
		var args struct {
			EventID  string `json:"event_id"`
			EntityID string `json:"entity_id"`
			Role     string `json:"role"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "create_event_entity"), &args)
		if args.EntityID != entityID.String() {
			t.Fatalf("ReferenceError: entity_id=%s", args.EntityID)
		}
		if args.EventID != ev.ID.String() {
			t.Fatalf("ReferenceError: event_id=%s want %s", args.EventID, ev.ID)
		}
		if !strings.EqualFold(args.Role, "witness") {
			t.Fatalf("ArgumentError: role=%q", args.Role)
		}
		if countEntities(t) != 1 {
			t.Fatalf("want 1 entity link, got %d", countEntities(t))
		}
	})

	t.Run("known_at_then_belief_role_details", func(t *testing.T) {
		const knownAtTicks repositories.Ticks = 1728000 // 20 days × 86400 s; 1 tick = 1 second
		resetToolSeq()
		invoke(t, fmt.Sprintf(
			`Для персонажа %s и события %s:
1) Персонаж узнал о событии ровно через 20 суток после начала кампании; укажи это с убеждением 80;
2) затем измени убеждение на 55;
3) смени роль на actor;
4) поставь details {"source":"rumor"}.`,
			entityID.String(), ev.ID.String(),
		))
		seq := snapshotToolSeq()
		requireTool(t, seq, "update_event_entity_known_at")
		requireTool(t, seq, "update_event_entity_belief")
		requireTool(t, seq, "update_event_entity_role")
		requireTool(t, seq, "update_event_entity_details")

		var knownArgs struct {
			KnownAtGameTime repositories.Ticks `json:"known_at_game_time"`
			Belief          int8               `json:"belief"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "update_event_entity_known_at"), &knownArgs)
		if knownArgs.KnownAtGameTime != knownAtTicks {
			t.Fatalf("ArgumentError: known_at=%d want %d", knownArgs.KnownAtGameTime, knownAtTicks)
		}

		got, err := testRepo.GetEventEntity(context.Background(), entityID, ev.ID)
		if err != nil {
			t.Fatalf("GetEventEntity: %v", err)
		}
		if got.KnownAtGameTime == nil || *got.KnownAtGameTime != knownAtTicks {
			t.Fatalf("known_at=%v want %d", got.KnownAtGameTime, knownAtTicks)
		}
		if got.Belief == nil || *got.Belief != 55 {
			t.Fatalf("belief=%v", got.Belief)
		}
		if got.Role != repoevents.EventEntityRoleActor {
			t.Fatalf("role=%s", got.Role)
		}
		if !strings.Contains(string(got.Details), "rumor") {
			t.Fatalf("details=%s", got.Details)
		}
	})
}

func TestInvoke_EventCausal_CreateUpdateFind(t *testing.T) {
	t.Run("create_and_update_cause", func(t *testing.T) {
		resetDB(t)
		a := seedEvent(t, "Барон приказал казнить мельника", 70, 10)
		b := seedEvent(t, "Сын мельника поклялся отомстить", 80, 20)

		invoke(t, fmt.Sprintf(
			`Укажи, что событие A (%s) стало причиной события B (%s). Причина: сын решил отомстить за казнь отца.
Затем измени текст причины на: "клятва мести после казни".`,
			a.ID.String(), b.ID.String(),
		))
		seq := snapshotToolSeq()
		requireTool(t, seq, "create_event_causal")
		requireTool(t, seq, "update_event_causal_cause")

		var createArgs struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Cause string `json:"cause"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "create_event_causal"), &createArgs)
		if createArgs.From != a.ID.String() || createArgs.To != b.ID.String() {
			t.Fatalf("ReferenceError: from→to = %s→%s", createArgs.From, createArgs.To)
		}
		if strings.TrimSpace(createArgs.Cause) == "" {
			t.Fatal("ArgumentError: empty cause")
		}

		var upd struct {
			From  string `json:"from"`
			To    string `json:"to"`
			Cause string `json:"cause"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "update_event_causal_cause"), &upd)
		if upd.From != a.ID.String() || upd.To != b.ID.String() {
			t.Fatalf("ReferenceError on update: %s→%s", upd.From, upd.To)
		}

		edges := listCausals(t)
		if len(edges) != 1 || edges[0].From != a.ID || edges[0].To != b.ID {
			t.Fatalf("DB edges: %#v", edges)
		}
		if edges[0].Cause != upd.Cause {
			t.Fatalf("DB cause=%q update_args=%q", edges[0].Cause, upd.Cause)
		}
	})

	t.Run("find_causes_of_C", func(t *testing.T) {
		resetDB(t)
		a := seedEvent(t, "Барон приказал казнить мельника", 70, 10)
		b := seedEvent(t, "Сын мельника поклялся отомстить", 80, 20)
		c := seedEvent(t, "Сын вступил в шайку разбойников", 60, 30)
		if err := testRepo.CreateEventCausal(context.Background(), &repoevents.EventCausal{From: a.ID, To: b.ID, Cause: "A→B"}); err != nil {
			t.Fatalf("seed A→B: %v", err)
		}
		if err := testRepo.CreateEventCausal(context.Background(), &repoevents.EventCausal{From: b.ID, To: c.ID, Cause: "B→C"}); err != nil {
			t.Fatalf("seed B→C: %v", err)
		}

		invokeID := invoke(t, fmt.Sprintf(
			`Найди причины события C (%s). Одним send_message укажи from-id прямой причины.`,
			c.ID.String(),
		))
		seq := snapshotToolSeq()
		requireTool(t, seq, "find_event_causals")
		var args struct {
			To      *string `json:"to"`
			EventID *string `json:"event_id"`
			From    *string `json:"from"`
		}
		unmarshalArgs(t, firstToolArgs(seq, "find_event_causals"), &args)
		if args.To == nil && args.EventID == nil {
			t.Fatal("ArgumentError: find_event_causals without to/event_id")
		}
		if text, ok := lastPlayerMessage(t, invokeID); ok {
			if !strings.Contains(text, b.ID.String()) {
				t.Fatalf("ToolResultHandlingError: message should mention cause B=%s, got %q", b.ID, text)
			}
		}
	})
}

func TestInvoke_MultiTool_TaxThenRevoltCausal(t *testing.T) {
	resetDB(t)

	invoke(t, `Создай два события: «Барон объявил новый налог» и «Крестьяне подняли восстание».
created_at_game_time налога = 40, восстания = 45 (ticks). Важность обоих — 60.
Затем укажи, что введение налога стало причиной восстания (from=налог, to=восстание).`)

	seq := snapshotToolSeq()
	if countTool(seq, "create_event") < 2 {
		t.Fatalf("IncompleteWorkflow: create_event calls=%d seq=%v", countTool(seq, "create_event"), toolNames(seq))
	}
	requireTool(t, seq, "create_event_causal")

	events := listEvents(t)
	if len(events) != 2 {
		t.Fatalf("want 2 events, got %d (%#v)", len(events), events)
	}
	tax := findEventBySummarySubstr(t, "налог")
	revolt := findEventBySummarySubstr(t, "восстан")
	if tax == nil || revolt == nil {
		t.Fatalf("could not identify tax/revolt events: %#v", events)
	}

	var causalArgs struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	unmarshalArgs(t, firstToolArgs(seq, "create_event_causal"), &causalArgs)
	if causalArgs.From != tax.ID.String() || causalArgs.To != revolt.ID.String() {
		t.Fatalf("ReferenceError: causal %s→%s want %s→%s", causalArgs.From, causalArgs.To, tax.ID, revolt.ID)
	}
	if countCausals(t) != 1 {
		t.Fatalf("want 1 causal, got %d", countCausals(t))
	}
	edge := listCausals(t)[0]
	if edge.From != tax.ID || edge.To != revolt.ID {
		t.Fatalf("DB causal direction wrong: %+v", edge)
	}
}

func TestInvoke_CausalCycle_ThenSelfLinkRejected(t *testing.T) {
	resetDB(t)

	a := seedEvent(t, "Event A", 50, 1)
	b := seedEvent(t, "Event B", 50, 2)
	c := seedEvent(t, "Event C", 50, 3)
	_ = testRepo.CreateEventCausal(context.Background(), &repoevents.EventCausal{From: a.ID, To: b.ID, Cause: "A→B"})
	_ = testRepo.CreateEventCausal(context.Background(), &repoevents.EventCausal{From: b.ID, To: c.ID, Cause: "B→C"})

	resetToolSeq()
	invoke(t, fmt.Sprintf(
		`Добавь причинную связь от C (%s) к A (%s) с причиной "цикл допустим".`,
		c.ID.String(), a.ID.String(),
	))
	seq := snapshotToolSeq()
	requireTool(t, seq, "create_event_causal")
	if countCausals(t) != 3 {
		t.Fatalf("cycle edge missing, causals=%d", countCausals(t))
	}

	resetToolSeq()
	invokeID := invoke(t, fmt.Sprintf(
		`Создай причинную связь от A (%s) к самому A. Если инструмент вернул ошибку — сообщи об отказе через send_message.`,
		a.ID.String(),
	))
	seq = snapshotToolSeq()
	requireTool(t, seq, "create_event_causal")
	if countCausals(t) != 3 {
		t.Fatalf("self-link must not persist, causals=%d", countCausals(t))
	}
	if !journalHasFailed(t, invokeID, "create_event_causal") {
		// Model might validate client-side without calling — still OK if no new edge;
		// but if it called, journal should show failure.
		if countTool(seq, "create_event_causal") > 0 {
			t.Fatal("ExecutionError: self-link call must fail in journal")
		}
	}
	if text, ok := lastPlayerMessage(t, invokeID); ok {
		lower := strings.ToLower(text)
		if strings.Contains(lower, "успеш") && !strings.Contains(lower, "ошиб") && !strings.Contains(lower, "нельзя") && !strings.Contains(lower, "reject") {
			t.Fatalf("FalseSuccess after self-link: %q", text)
		}
	}
}

func TestInvoke_ErrorHandling_DuplicateAndBadArgs(t *testing.T) {
	resetDB(t)

	a := seedEvent(t, "Event A", 50, 1)
	b := seedEvent(t, "Event B", 50, 2)
	_ = testRepo.CreateEventCausal(context.Background(), &repoevents.EventCausal{
		From: a.ID, To: b.ID, Cause: "already exists",
	})

	invokeID := invoke(t, fmt.Sprintf(
		`Попробуй снова создать причинную связь from=%s to=%s с причиной "duplicate".
Если ошибка — опиши её в send_message. Не утверждай успех.`,
		a.ID.String(), b.ID.String(),
	))
	if countCausals(t) != 1 {
		t.Fatalf("duplicate must not add row, count=%d", countCausals(t))
	}
	seq := snapshotToolSeq()
	if countTool(seq, "create_event_causal") >= 1 && !journalHasFailed(t, invokeID, "create_event_causal") {
		t.Fatal("duplicate create_event_causal should fail in journal when called")
	}

	resetToolSeq()
	invoke(t, `Создай событие с importance=255 и summary "bad". Если аргумент отклонён — не создавай запись и сообщи об ошибке через send_message.`)
	// Schema/service should reject; DB must stay clean of the bad event.
	for _, ev := range listEvents(t) {
		if ev.Summary == "bad" {
			t.Fatalf("bad importance event persisted: %+v", ev)
		}
	}
}
