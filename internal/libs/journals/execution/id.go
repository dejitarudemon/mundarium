package journal

import "github.com/dejitarudemon/mundarium/internal/libs/types"

// ID identifies a journaled request or invoke/group.
// It is an alias of types.ID so journal and execution share one identity type.
type ID = types.ID
