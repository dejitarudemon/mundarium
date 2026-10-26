package errs

import (
	"errors"

	svcerrs "github.com/dejitarudemon/mundarium/internal/services/errs"
)

// From maps err to a ToolErr.
//
// nil err returns nil. Values that already implement ToolErr are returned as-is.
// Service-layer [svcerrs.ServiceError] values become [ServiceError].
// Everything else becomes [InternalError].
func From(err error) error {
	if err == nil {
		return nil
	}

	if te, ok := errors.AsType[ToolErr](err); ok {
		return te
	}

	if se, ok := errors.AsType[svcerrs.ServiceError](err); ok {
		return NewServiceError(se)
	}

	return NewInternalError(err)
}
