package journal_test

import (
	"testing"

	journal "github.com/dejitarudemon/mundarium/internal/libs/journals/execution"
)

func TestStatus_String(t *testing.T) {
	if got := journal.Status_Running.String(); got != "running" {
		t.Fatalf("Status_Running.String: want running, got %q", got)
	}
}
