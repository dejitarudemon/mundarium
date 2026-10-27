package locations

import (
	"errors"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	repolocations "github.com/dejitarudemon/mundarium/internal/repositories/locations"
	"github.com/dejitarudemon/mundarium/internal/services/errs"
)

const (
	locationErrEntity           = "location"
	locationSubtypeErrEntity    = "location_subtype"
	locationConnectionErrEntity = "location_connection"
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

// mapLocationErr maps storage and domain failures for a location id to ServiceError.
func mapLocationErr(err error, id repositories.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	if errors.Is(err, repolocations.ErrLocationWouldCreateCycle) {
		return errs.NewInvalidRequest(err.Error())
	}
	return errs.From(err, locationErrEntity, fmt.Sprintf("%d", id))
}

// mapSubtypeErr maps storage and domain failures for a subtype id to ServiceError.
func mapSubtypeErr(err error, id repositories.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	return errs.From(err, locationSubtypeErrEntity, fmt.Sprintf("%d", id))
}

// mapConnectionErr maps storage and domain failures for a connection id to ServiceError.
func mapConnectionErr(err error, id repositories.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	return errs.From(err, locationConnectionErrEntity, fmt.Sprintf("%d", id))
}
