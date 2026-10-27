package messages

import (
	"errors"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/repositories"
	"github.com/dejitarudemon/mundarium/internal/services/errs"
)

const messageErrEntity = "message"

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

// mapMessageErr maps storage failures for a message id to ServiceError.
func mapMessageErr(err error, id repositories.ID) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	return errs.From(err, messageErrEntity, fmt.Sprintf("%d", id))
}

// mapMessageKeyErr maps storage failures labeled by an opaque key (e.g. invoke id).
func mapMessageKeyErr(err error, key string) error {
	if err == nil {
		return nil
	}
	if se, ok := errors.AsType[errs.ServiceError](err); ok {
		return se
	}
	return errs.From(err, messageErrEntity, key)
}
