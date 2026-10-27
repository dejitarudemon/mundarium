package errs

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

// From maps err to a ServiceError.
//
// entity and id label the resource for NotFound and AlreadyCreated.
// nil err returns nil. Values that already implement ServiceError are returned as-is.
// Known gorm sentinel errors and PostgreSQL constraint codes are classified;
// everything else becomes InternalError.
func From(err error, entity, id string) error {
	if err == nil {
		return nil
	}

	if se, ok := errors.AsType[ServiceError](err); ok {
		return se
	}

	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return NewNotFound(entity, id)
	case errors.Is(err, gorm.ErrDuplicatedKey), postgresCode(err) == "23505":
		return NewAlreadyCreated(entity, id)
	case errors.Is(err, gorm.ErrForeignKeyViolated),
		errors.Is(err, gorm.ErrCheckConstraintViolated),
		errors.Is(err, gorm.ErrInvalidData),
		errors.Is(err, gorm.ErrInvalidField),
		errors.Is(err, gorm.ErrInvalidValue),
		errors.Is(err, gorm.ErrPrimaryKeyRequired),
		postgresCode(err) == "23503", // foreign_key_violation
		postgresCode(err) == "23514": // check_violation
		return NewInvalidRequest(err.Error())
	default:
		return NewInternalError(err)
	}
}

// postgresCode returns the SQLSTATE from a pgx PgError in err's chain, if any.
func postgresCode(err error) string {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code
	}
	return ""
}
