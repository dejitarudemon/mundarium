package postgres

import (
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

func TestEmergency_AddFindDelete(t *testing.T) {
	e := newEmergency()
	id, group := journal.ID("req-1"), journal.ID("grp-1")
	other := journal.ID("grp-2")

	if e.find(id, group) {
		t.Fatal("empty emergency must miss")
	}

	e.add(id, group)
	if !e.find(id, group) {
		t.Fatal("want find after add")
	}
	if e.find(id, other) {
		t.Fatal("must not match other group")
	}

	e.delete(group)
	if e.find(id, group) {
		t.Fatal("want miss after delete group")
	}
}
