package errs_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dejitarudemon/mundarium/internal/services/errs"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

func TestFrom_Nil(t *testing.T) {
	if got := errs.From(nil, "location", "1"); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
}

func TestFrom_PassthroughServiceError(t *testing.T) {
	tb := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	src := errs.NewNotFoundWithTracebackID("location", "1", tb)

	got := errs.From(src, "ignored", "ignored")
	nf, ok := got.(errs.NotFound)
	if !ok {
		t.Fatalf("want NotFound, got %T", got)
	}
	if nf.TracebackID() != tb || nf.Entity != "location" || nf.ID != "1" {
		t.Fatalf("passthrough mutated: %+v", nf)
	}
}

func TestFrom_GormRecordNotFound(t *testing.T) {
	got := errs.From(gorm.ErrRecordNotFound, "location", "42")
	nf, ok := got.(errs.NotFound)
	if !ok {
		t.Fatalf("want NotFound, got %T (%v)", got, got)
	}
	if nf.Entity != "location" || nf.ID != "42" {
		t.Fatalf("unexpected NotFound: %+v", nf)
	}
	if nf.TracebackID() == uuid.Nil {
		t.Fatal("want traceback id")
	}
}

func TestFrom_GormDuplicatedKey(t *testing.T) {
	got := errs.From(gorm.ErrDuplicatedKey, "message", "7")
	ac, ok := got.(errs.AlreadyCreated)
	if !ok {
		t.Fatalf("want AlreadyCreated, got %T (%v)", got, got)
	}
	if ac.Entity != "message" || ac.ID != "7" {
		t.Fatalf("unexpected AlreadyCreated: %+v", ac)
	}
}

func TestFrom_GormInvalidRequestClass(t *testing.T) {
	got := errs.From(gorm.ErrForeignKeyViolated, "location", "1")
	ir, ok := got.(errs.InvalidRequest)
	if !ok {
		t.Fatalf("want InvalidRequest, got %T (%v)", got, got)
	}
	if ir.Reason == "" {
		t.Fatal("want non-empty reason")
	}
}

func TestFrom_PostgresForeignKeyViolation23503(t *testing.T) {
	cause := &pgconn.PgError{
		Code:    "23503",
		Message: `insert or update on table "locations" violates foreign key constraint "fk_locations_subtype"`,
	}
	// Driver path without gorm.Config.TranslateError, and wrapped like gorm Translate.
	for _, err := range []error{
		cause,
		fmt.Errorf("db: %w", cause),
		fmt.Errorf("%w: %w", gorm.ErrForeignKeyViolated, cause),
	} {
		got := errs.From(err, "location", "1")
		if _, ok := got.(errs.InvalidRequest); !ok {
			t.Fatalf("SQLSTATE 23503 want InvalidRequest, got %T (%v)", got, got)
		}
	}
}

func TestFrom_UnknownIsInternal(t *testing.T) {
	cause := errors.New("connection reset")
	got := errs.From(cause, "location", "1")
	ie, ok := got.(errs.InternalError)
	if !ok {
		t.Fatalf("want InternalError, got %T (%v)", got, got)
	}
	if !errors.Is(ie, cause) {
		t.Fatalf("want unwrap to %v", cause)
	}
}
