package events

import "fmt"

// EventEntityRole is the role of an entity relative to an event or knowledge of it.
type EventEntityRole uint16

const (
	EventEntityRoleUnknown EventEntityRole = iota

	EventEntityRoleActor       // active participant who performs the event
	EventEntityRoleParticipant // participant in the event
	EventEntityRoleAffected    // entity the event is about or that was affected by it
	EventEntityRoleWitness     // direct witness
	EventEntityRoleInformed    // entity informed about the event
)

var fromMapRoles = map[string]EventEntityRole{
	"unknown":     EventEntityRoleUnknown,
	"actor":       EventEntityRoleActor,
	"participant": EventEntityRoleParticipant,
	"affected":    EventEntityRoleAffected,
	"witness":     EventEntityRoleWitness,
	"informed":    EventEntityRoleInformed,
}

// NewEventEntityRole parses s into an EventEntityRole.
// Unknown values map to EventEntityRoleUnknown.
func NewEventEntityRole(s string) (EventEntityRole, error) {
	if r, ok := fromMapRoles[s]; ok {
		return r, nil
	}
	return EventEntityRoleUnknown, fmt.Errorf("unexpected event entity role: %v", s)
}

// String returns the canonical name of the role.
func (r EventEntityRole) String() string {
	switch r {
	case EventEntityRoleActor:
		return "actor"
	case EventEntityRoleParticipant:
		return "participant"
	case EventEntityRoleAffected:
		return "affected"
	case EventEntityRoleWitness:
		return "witness"
	case EventEntityRoleInformed:
		return "informed"
	case EventEntityRoleUnknown:
		return "unknown"
	}
	return fmt.Sprintf("unexpected role (%d)", r)
}
