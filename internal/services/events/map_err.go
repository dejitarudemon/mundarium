package events

import (
	"errors"
	"fmt"

	repoevents "github.com/dejitarudemon/mundarium/internal/repositories/events"
	"github.com/dejitarudemon/mundarium/internal/services/errs"
)

const (
	eventErrEntity       = "event"
	eventEntityErrEntity = "event_entity"
	eventCausalErrEntity = "event_causal"
)

// mapRequestErr maps input/validation failures to InvalidRequest.
func mapRequestErr(err error) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	return errs.NewInvalidRequest(err.Error())
}

// mapEventErr maps storage and domain failures for an event id to ServiceError.
func mapEventErr(err error, id repoevents.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	switch {
	case errors.Is(err, repoevents.ErrEventImportanceOutOfRange),
		errors.Is(err, repoevents.ErrEventLocationRequired),
		errors.Is(err, repoevents.ErrEventCausalSelfLink),
		errors.Is(err, repoevents.ErrEventCausalIncomplete):
		return errs.NewInvalidRequest(err.Error())
	}
	return errs.From(err, eventErrEntity, id.String())
}

// mapEventEntityErr maps storage and domain failures for an entity–event link to ServiceError.
func mapEventEntityErr(err error, entityID, eventID string) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	switch {
	case errors.Is(err, repoevents.ErrEventEntityAlreadyKnown),
		errors.Is(err, repoevents.ErrEventEntityNotKnown):
		return errs.NewInvalidRequest(err.Error())
	}
	return errs.From(err, eventEntityErrEntity, fmt.Sprintf("%s/%s", entityID, eventID))
}

// mapEventCausalErr maps storage and domain failures for a causal edge to ServiceError.
func mapEventCausalErr(err error, from, to repoevents.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	switch {
	case errors.Is(err, repoevents.ErrEventCausalSelfLink),
		errors.Is(err, repoevents.ErrEventCausalIncomplete):
		return errs.NewInvalidRequest(err.Error())
	}
	return errs.From(err, eventCausalErrEntity, fmt.Sprintf("%s->%s", from, to))
}
