package implemented

import (
	"errors"
	"fmt"
)

var (
	ErrNoExecutor = errors.New("tools executor is nil")
	ErrNoTools    = errors.New("no allowed tools")
)

// ProviderError is a provider-domain error that reports whether the call may be retried.
type ProviderError interface {
	error

	IsRepeatable() bool
}

// ProviderNotAvailableError indicates the remote provider could not be reached or answered.
type ProviderNotAvailableError struct {
	code  int
	extra map[string]any
}

// NewProviderNotAvailableError creates a ProviderNotAvailableError for an HTTP status code.
func NewProviderNotAvailableError(code int, extra map[string]any) ProviderNotAvailableError {
	return ProviderNotAvailableError{
		code:  code,
		extra: extra,
	}
}

func (e ProviderNotAvailableError) Error() string {
	return fmt.Sprintf("provider not available (%v code): %v", e.code, e.extra)
}

// IsRepeatable reports that a missing provider should not be retried blindly.
func (e ProviderNotAvailableError) IsRepeatable() bool {
	return false
}

// InternalError wraps an unexpected provider failure.
type InternalError struct {
	err   error
	extra map[string]any
}

// NewInternalError creates an InternalError around err.
func NewInternalError(err error, extra map[string]any) InternalError {
	return InternalError{
		err:   err,
		extra: extra,
	}
}

func (e InternalError) Error() string {
	return fmt.Sprintf("internal error (%v): %v", e.err, e.extra)
}

// Unwrap returns the underlying error.
func (e InternalError) Unwrap() error {
	return e.err
}

// IsRepeatable reports that transient internal failures may be retried.
func (e InternalError) IsRepeatable() bool {
	return true
}

// NoModelError indicates the configured model id is missing on the provider.
type NoModelError struct {
	id    string
	extra map[string]any
}

// NewNoModelError creates a NoModelError for id.
func NewNoModelError(id string, extra map[string]any) NoModelError {
	return NoModelError{
		id:    id,
		extra: extra,
	}
}

func (e NoModelError) Error() string {
	return fmt.Sprintf("no model %v: %v", e.id, e.extra)
}

// IsRepeatable reports that a missing model should not be retried.
func (e NoModelError) IsRepeatable() bool {
	return false
}

// TimeoutError indicates the provider call exceeded its deadline.
type TimeoutError struct {
	extra map[string]any
}

// NewTimeoutError creates a TimeoutError.
func NewTimeoutError(extra map[string]any) TimeoutError {
	return TimeoutError{
		extra: extra,
	}
}

func (e TimeoutError) Error() string {
	return fmt.Sprintf("ctx cancelled (timeout): %v", e.extra)
}

// IsRepeatable reports that timeouts may be retried.
func (e TimeoutError) IsRepeatable() bool {
	return true
}

// InvalidRequestError indicates the provider rejected the request as invalid.
type InvalidRequestError struct {
	extra map[string]any
}

// NewInvalidRequestError creates an InvalidRequestError.
func NewInvalidRequestError(extra map[string]any) InvalidRequestError {
	return InvalidRequestError{
		extra: extra,
	}
}

func (e InvalidRequestError) Error() string {
	return fmt.Sprintf("invalid request: %v", e.extra)
}

// IsRepeatable reports that invalid requests should not be retried.
func (e InvalidRequestError) IsRepeatable() bool {
	return false
}

// NotAuthorizedError indicates authentication or authorization failed.
type NotAuthorizedError struct {
	extra map[string]any
}

// NewNotAuthorizedError creates a NotAuthorizedError.
func NewNotAuthorizedError(extra map[string]any) NotAuthorizedError {
	return NotAuthorizedError{
		extra: extra,
	}
}

func (e NotAuthorizedError) Error() string {
	return fmt.Sprintf("not authorized: %v", e.extra)
}

// IsRepeatable reports that auth failures should not be retried.
func (e NotAuthorizedError) IsRepeatable() bool {
	return false
}

// TooManyRequestsError indicates the provider rate-limited the caller.
type TooManyRequestsError struct {
	extra map[string]any
}

// NewTooManyRequestsError creates a TooManyRequestsError.
func NewTooManyRequestsError(extra map[string]any) TooManyRequestsError {
	return TooManyRequestsError{
		extra: extra,
	}
}

func (e TooManyRequestsError) Error() string {
	return fmt.Sprintf("too many requests: %v", e.extra)
}

// IsRepeatable reports that rate limits may be retried after backing off.
func (e TooManyRequestsError) IsRepeatable() bool {
	return true
}
