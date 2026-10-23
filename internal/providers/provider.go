// Package providers defines outbound provider integrations.
package providers

import (
	"context"

	libproviders "github.com/dejitarudemon/mundarium/internal/libs/providers"
)

// Provider invokes an external capability identified by id with the given request.
type Provider interface {
	Invoke(ctx context.Context, id string, request libproviders.Request) error
}
