package openai

import (
	"context"
	"encoding/json"

	"github.com/dejitarudemon/mundarium/internal/providers"
	"github.com/dejitarudemon/mundarium/internal/providers/implemented"
	"github.com/dejitarudemon/mundarium/internal/tools"
	"github.com/openai/openai-go/v3"
)

func (p *OpenAICompatibleProvider) Invoke(ctx context.Context, id string, request providers.Request) error {
	history := []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(p.prompt),
		openai.UserMessage(string(request.Args)),
	}

	for range p.maxIters {
		response, err := p.send(ctx, history)
		if err != nil {
			return err
		}

		if len(response.ToolCalls) == 0 {
			return nil
		}

		history = append(history, response.ToParam())

		for _, tool := range response.ToolCalls {
			answer, err := p.executor.Execute(ctx, id, tools.Call{
				ID:   tool.ID,
				Name: tool.Function.Name,
				Role: p.role,
				Args: json.RawMessage(tool.Function.Arguments),
			})
			if err != nil {
				history = append(history, NewFailToolResponse(tool.ID, err))
				continue
			}
			history = append(history, NewSuccessfulToolResponse(tool.ID, answer))
		}
	}
	return implemented.ErrTooManyIterations
}
