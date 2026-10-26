package journal_test

import (
	"testing"

	"github.com/dejitarudemon/mundarium/internal/journal"
)

func TestAction_String(t *testing.T) {
	if got := journal.Action_Create.String(); got != "create" {
		t.Fatalf("Action_Create.String: want create, got %q", got)
	}
}
