// Package providers defines outbound provider integrations.
package providers

import (
	"context"
)

// Provider invokes an external capability identified by id with the given request.
type Provider interface {
	Invoke(ctx context.Context, id string, request Request) error
}
