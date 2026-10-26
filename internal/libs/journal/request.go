package journal

import "time"

// Request is a journaled tool invocation and its outcome.
// Result is set when Status is completed; Error is set when Status is failed.
type Request struct {
	ID      ID `gorm:"primaryKey;type:text" json:"id"`
	GroupID ID `gorm:"primaryKey;type:text;index" json:"group_id"`

	CreatedAt time.Time  `gorm:"not null" json:"created_at"`
	ExpiresAt *time.Time `gorm:"index" json:"expires_at,omitempty"`

	Status   Status `gorm:"not null;type:text" json:"status"`
	Tool     string `gorm:"not null;type:text" json:"tool"` // tool name, e.g. "create_location"
	ArgsHash []byte `gorm:"type:bytea" json:"args_hash"`    // blake2b digest of tool args; computed by the executor

	Result Result    `gorm:"serializer:json;type:jsonb" json:"result"`
	Error  ErrorText `gorm:"column:error;type:text" json:"error,omitempty"`
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
