package journal

import (
	"database/sql/driver"
	"fmt"
)

// ErrorText is a persisted error message.
// Arbitrary Go error types cannot round-trip through the database; only the message is kept.
type ErrorText string

// ErrorTextFrom stores err's message, or empty if err is nil.
func ErrorTextFrom(err error) ErrorText {
	if err == nil {
		return ""
	}
	return ErrorText(err.Error())
}

// Err returns a non-nil error when the message is non-empty.
func (e ErrorText) Err() error {
	if e == "" {
		return nil
	}
	return e
}

// Error implements the error interface.
func (e ErrorText) Error() string {
	return string(e)
}

// Value implements driver.Valuer for GORM/SQL storage.
func (e ErrorText) Value() (driver.Value, error) {
	if e == "" {
		return nil, nil
	}
	return string(e), nil
}

// Scan implements sql.Scanner for GORM/SQL loading.
func (e *ErrorText) Scan(value any) error {
	if e == nil {
		return fmt.Errorf("journal.ErrorText: Scan on nil pointer")
	}
	if value == nil {
		*e = ""
		return nil
	}
	switch v := value.(type) {
	case string:
		*e = ErrorText(v)
	case []byte:
		*e = ErrorText(v)
	default:
		return fmt.Errorf("journal.ErrorText: cannot scan %T", value)
	}
	return nil
}
