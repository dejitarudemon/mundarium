package events

import "fmt"

// EventCausal is a directed causal edge from one event to another.
// From is the cause; To is the effect. Direct self-links (From == To) are forbidden.
type EventCausal struct {
	From ID `gorm:"column:from_id;type:uuid;primaryKey;check:chk_event_causals_no_self,from_id <> to_id" json:"from"`
	To   ID `gorm:"column:to_id;type:uuid;primaryKey;index" json:"to"`

	Cause string `gorm:"type:text;not null" json:"cause"`
}

// String returns a compact debug representation of the causal link.
func (c EventCausal) String() string {
	return fmt.Sprintf("EventCausal{From:%s To:%s Cause:%q}", c.From, c.To, c.Cause)
}
