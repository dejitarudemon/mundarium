package executor

import (
	"encoding/json"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/journal"
	libjournal "github.com/dejitarudemon/mundarium/internal/libs/journal"
)

func createRequestID(invokeID, callID string) libjournal.RequestID {
	return libjournal.RequestID(fmt.Sprintf("%s%s%s", invokeID, IDSep, callID))
}

func resultPayload(result journal.Result) (json.RawMessage, error) {
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
