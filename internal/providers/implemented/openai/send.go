package openai

import (
	"context"

	"github.com/openai/openai-go/v3"
)

// send performs one chat-completions round trip. Kept unexported until Invoke is wired.
//
//nolint:unused // used by the upcoming Invoke loop
func (p *OpenAICompatibleProvider) send(ctx context.Context, messages []openai.ChatCompletionMessageParamUnion) (openai.ChatCompletionMessage, error) {
	response, err := p.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Model:    p.model,
			Messages: messages,
			Tools:    p.tools,
		},
	)
	if err != nil {
		return openai.ChatCompletionMessage{}, p.handleError(err)
	}

	if len(response.Choices) == 0 {
		return openai.ChatCompletionMessage{}, p.handleError(ErrNoChoices)
	}

	choice := response.Choices[0]

	if choice.FinishReason == "length" {
		return openai.ChatCompletionMessage{}, p.handleError(ErrTruncated)
	}

	return choice.Message, nil
}
