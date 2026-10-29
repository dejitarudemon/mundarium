package events_test

import (
	"errors"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/repositories/entities"
	"github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
	"gorm.io/gorm"
)

func ptrInt8(v int8) *int8 { return &v }

func ptrTicks(v repositories.Ticks) *repositories.Ticks { return &v }

func isCheckViolation(err error) bool {
	if errors.Is(err, gorm.ErrCheckConstraintViolated) {
		return true
	}
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23514"
}

func createEvent(t *testing.T, importance int8, summary string) *events.Event {
	t.Helper()
	ev := &events.Event{
		Importance:        importance,
		CreatedAtGameTime: 100,
		Summary:           summary,
		Tags:              pq.StringArray{"test"},
	}
	if err := testDB.Create(ev).Error; err != nil {
		t.Fatalf("Create Event(%q): %v", summary, err)
	}
	return ev
}

func TestEvent_ID_SQLRoundTrip(t *testing.T) {
	resetDB(t)

	ev := createEvent(t, 50, "sql-id")
	if ev.ID == (events.ID{}) {
		t.Fatal("BeforeCreate must assign a non-nil ID")
	}

	var got events.Event
	if err := testDB.First(&got, "id = ?", ev.ID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.ID != ev.ID {
		t.Fatalf("ID round-trip: want %s, got %s", ev.ID, got.ID)
	}
	if got.Summary != "sql-id" || got.Importance != 50 {
		t.Fatalf("unexpected event: %+v", got)
	}
}

func TestEvent_Importance_Bounds(t *testing.T) {
	resetDB(t)

	for _, importance := range []int8{0, 100} {
		ev := &events.Event{
			Importance:        importance,
			CreatedAtGameTime: 1,
			Summary:           "ok",
		}
		if err := testDB.Create(ev).Error; err != nil {
			t.Fatalf("Create Importance=%d: %v", importance, err)
		}
	}
}

func TestEvent_Importance_OutOfRange(t *testing.T) {
	resetDB(t)

	for _, importance := range []int8{-1, 101} {
		ev := &events.Event{
			Importance:        importance,
			CreatedAtGameTime: 1,
			Summary:           "bad",
		}
		err := testDB.Create(ev).Error
		if !isCheckViolation(err) {
			t.Fatalf("Importance=%d: want check violation, got %v", importance, err)
		}
	}
}

func TestEventEntity_BeliefNilWithKnownAt(t *testing.T) {
	resetDB(t)

	ev := createEvent(t, 10, "known")
	link := &events.EventEntity{
		EntityID:        entities.New(),
		EventID:         ev.ID,
		Role:            events.EventEntityRoleWitness,
		Belief:          nil,
		KnownAtGameTime: ptrTicks(42),
	}
	if err := testDB.Create(link).Error; err != nil {
		t.Fatalf("Create with Belief=nil and KnownAtGameTime set: %v", err)
	}

	var got events.EventEntity
	if err := testDB.First(&got, "entity_id = ? AND event_id = ?", link.EntityID, link.EventID).Error; err != nil {
		t.Fatalf("First: %v", err)
	}
	if got.Belief != nil {
		t.Fatalf("want Belief=nil, got %v", *got.Belief)
	}
	if got.KnownAtGameTime == nil || *got.KnownAtGameTime != 42 {
		t.Fatalf("unexpected KnownAtGameTime: %v", got.KnownAtGameTime)
	}
}

func TestEventEntity_Belief_Bounds(t *testing.T) {
	resetDB(t)
	ev := createEvent(t, 10, "belief-ok")

	for _, belief := range []int8{0, 100} {
		link := &events.EventEntity{
			EntityID: entities.New(),
			EventID:  ev.ID,
			Role:     events.EventEntityRoleActor,
			Belief:   ptrInt8(belief),
		}
		if err := testDB.Create(link).Error; err != nil {
			t.Fatalf("Create Belief=%d: %v", belief, err)
		}
	}
}

func TestEventEntity_Belief_OutOfRange(t *testing.T) {
	resetDB(t)
	ev := createEvent(t, 10, "belief-bad")

	for _, belief := range []int8{-1, 101} {
		link := &events.EventEntity{
			EntityID: entities.New(),
			EventID:  ev.ID,
			Role:     events.EventEntityRoleActor,
			Belief:   ptrInt8(belief),
		}
		err := testDB.Create(link).Error
		if !isCheckViolation(err) {
			t.Fatalf("Belief=%d: want check violation, got %v", belief, err)
		}
	}
}

func TestEventCausal_RejectsSelfLink(t *testing.T) {
	resetDB(t)

	ev := createEvent(t, 5, "self")
	edge := &events.EventCausal{
		From:  ev.ID,
		To:    ev.ID,
		Cause: "loop",
	}
	err := testDB.Create(edge).Error
	if !isCheckViolation(err) {
		t.Fatalf("self-link: want check violation, got %v", err)
	}
}

func TestEventCausal_AllowsDistinctEndpoints(t *testing.T) {
	resetDB(t)

	cause := createEvent(t, 5, "cause")
	effect := createEvent(t, 5, "effect")
	edge := &events.EventCausal{
		From:  cause.ID,
		To:    effect.ID,
		Cause: "because",
	}
	if err := testDB.Create(edge).Error; err != nil {
		t.Fatalf("Create: %v", err)
	}
}

func TestEventCausal_ToIndexExists(t *testing.T) {
	var count int64
	err := testDB.Raw(`
		SELECT COUNT(*)
		FROM pg_indexes
		WHERE schemaname = 'public'
		  AND tablename = 'event_causals'
		  AND indexdef ILIKE '%(to_id)%'
	`).Scan(&count).Error
	if err != nil {
		t.Fatalf("query pg_indexes: %v", err)
	}
	if count < 1 {
		t.Fatal("expected an index on event_causals(to_id)")
	}
}
