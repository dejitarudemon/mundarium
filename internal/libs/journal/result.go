package journal

// Result describes a completed service operation for journaling or callbacks.
type Result struct {
	Action Action `gorm:"type:text" json:"action"`                 // see Action_*
	Data   any    `gorm:"serializer:json;type:jsonb" json:"data"` // operation payload
	Entity string `gorm:"type:text" json:"entity"`                 // entity kind, e.g. "LOCATION"
}
