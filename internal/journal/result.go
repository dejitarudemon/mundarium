package journal

import libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"

// Result describes a completed service operation for journaling or callbacks.
type Result struct {
	Action libjournal.Action // see libjournal.Action_*
	Data   any               // operation payload
	Entity string            // entity kind, e.g. "LOCATION"
}
