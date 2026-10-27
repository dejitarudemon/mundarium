package openai

import (
	"context"
	"errors"

	"github.com/dejitarudemon/mundarium/internal/providers/implemented"
	"github.com/openai/openai-go/v3"
)

var (
	ErrNoChoices = errors.New("no choices")
	ErrTruncated = errors.New("message truncated")
)

func (p *OpenAICompatibleProvider) baseExtra() map[string]any {
	return map[string]any{
		"type":  openAIProviderName,
		"model": p.model,
		"url":   p.url,
		"role":  p.role,
	}
}

func (p *OpenAICompatibleProvider) handleError(err error) error {
	base := p.baseExtra()

	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	if _, ok := err.(implemented.ProviderError); ok {
		return err
	}

	if apiErr, ok := errors.AsType[*openai.Error](err); ok {
		base["code"] = apiErr.Code

		switch apiErr.StatusCode {
		case 400:
			return implemented.NewInvalidRequestError(base)
		case 401, 403:
			return implemented.NewNotAuthorizedError(base)
		case 404:
			return implemented.NewNoModelError(p.model, base)
		case 408:
			return implemented.NewTimeoutError(base)
		case 429:
			return implemented.NewTooManyRequestsError(base)
		default:
			return implemented.NewInternalError(err, base)
		}
	}

	return implemented.NewInternalError(err, base)
}
