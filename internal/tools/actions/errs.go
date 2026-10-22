package actions

import "errors"

// ErrServiceIsNil is returned when a tool constructor receives a nil service dependency.
var ErrServiceIsNil = errors.New("expected service, got nil")
