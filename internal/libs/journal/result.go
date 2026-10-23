package journal

// Result describes a completed service operation for journaling or callbacks.
type Result struct {
	Action Action `json:"action"` // see Action_*
	Data   any    `json:"data"`   // operation payload
	Entity string `json:"entity"` // entity kind, e.g. "LOCATION"
}
