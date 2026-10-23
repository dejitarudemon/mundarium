package journal

import "time"

// Request is a journaled tool invocation and its outcome.
// Result is set when Status is completed; Error is set when Status is failed.
type Request struct {
	ID      RequestID `json:"id"`
	GroupID RequestID `json:"group_id"`

	CreatedAt time.Time  `json:"created_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	Status   Status `json:"status"`
	Tool     string `json:"tool"`      // tool name, e.g. "create_location"
	ArgsHash []byte `json:"args_hash"` // blake2b digest of tool args; computed by the executor

	Result Result `json:"result"`
	Error  error  `json:"error,omitempty"`
}

// IsExpired reports whether the request is past its soft ExpiresAt and/or hardTTL.
// Soft expiry: ExpiresAt is set and at or before now.
// Hard TTL expiry: CreatedAt + hardTTL is at or before now (hardTTL must be > 0).
func (r Request) IsExpired(now time.Time, hardTTL time.Duration) bool {
	if r.ExpiresAt != nil && !r.ExpiresAt.After(now) {
		return true
	}
	if hardTTL > 0 && !r.CreatedAt.IsZero() && !r.CreatedAt.Add(hardTTL).After(now) {
		return true
	}
	return false
}
