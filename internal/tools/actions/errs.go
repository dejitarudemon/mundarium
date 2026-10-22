package actions

import "errors"

// ServiceIsNil is returned when a tool constructor receives a nil service dependency.
var ServiceIsNil = errors.New("expected service, got nil")
