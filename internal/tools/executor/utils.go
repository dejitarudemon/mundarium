package executor

import (
	"encoding/json"

	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
	"golang.org/x/crypto/blake2b"
)

func toJournalID(id string) libjournal.ID {
	return libjournal.ID(id)
}

// hashArgs returns a blake2b-256 digest of tool arguments for semantic journal checks.
func hashArgs(args json.RawMessage) []byte {
	sum := blake2b.Sum256(args)
	out := make([]byte, len(sum))
	copy(out, sum[:])
	return out
}

func resultPayload(result libjournal.Result) (json.RawMessage, error) {
	if result.Data == nil {
		return json.RawMessage("null"), nil
	}
	switch data := result.Data.(type) {
	case json.RawMessage:
		return data, nil
	case []byte:
		return json.RawMessage(data), nil
	default:
		return json.Marshal(data)
	}
}
