package journal_test

import (
	"testing"

	"github.com/dejitarudemon/mundarium/internal/libs/journal"
)

func TestStatus_String(t *testing.T) {
	if got := journal.Status_Running.String(); got != "running" {
		t.Fatalf("Status_Running.String: want running, got %q", got)
	}
}
