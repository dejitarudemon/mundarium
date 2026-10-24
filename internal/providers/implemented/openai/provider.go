package openai

import (
	"context"
	"io"
	"net/http"

	libprovider "github.com/dejitarudemon/mundarium/internal/libs/providers"
	libopenai "github.com/dejitarudemon/mundarium/internal/libs/providers/openai"
	"github.com/dejitarudemon/mundarium/internal/providers/implemented"
	"github.com/dejitarudemon/mundarium/internal/tools/executor"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
)

const (
	openAIProviderName = "OpenAI provider"
)

// OpenAICompatibleProvider talks to an OpenAI-compatible chat completions API.
type OpenAICompatibleProvider struct {
	client     openai.Client
	httpClient *http.Client
	executor   *executor.Executor

	model  string
	prompt string
	url    string
	apiKey string

	role libprovider.Role

	tools []openai.ChatCompletionToolUnionParam
}

// NewOpenAICompatibleProvider builds a provider bound to executor tools allowed for role.
func NewOpenAICompatibleProvider(executor *executor.Executor, url, model, apiKey, prompt string, role libprovider.Role) (*OpenAICompatibleProvider, error) {
	if executor == nil {
		return nil, implemented.ErrNoExecutor
	}

	tools := executor.Allowed(role)
	if len(tools) == 0 {
		return nil, implemented.ErrNoTools
	}

	converted, err := convertTools(tools)
	if err != nil {
		return nil, err
	}

	client := openai.NewClient(
		option.WithBaseURL(url),
		option.WithAPIKey(apiKey),
	)

	return &OpenAICompatibleProvider{
		client:     client,
		httpClient: http.DefaultClient,
		model:      model,
		prompt:     prompt,
		url:        url,
		apiKey:     apiKey,
		role:       role,
		tools:      converted,
		executor:   executor,
	}, nil
}

// Available checks that the remote server answers /v1/models and lists the configured model.
func (p *OpenAICompatibleProvider) Available(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint(p.url, "/", endpoint_models), nil)
	if err != nil {
		return p.handleError(err)
	}
	if p.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+p.apiKey)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return p.handleError(err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return p.handleError(err)
	}

	if resp.StatusCode != http.StatusOK {
		return implemented.NewProviderNotAvailableError(resp.StatusCode, p.baseExtra())
	}

	models, err := libopenai.NewModelsResponse(body)
	if err != nil {
		return p.handleError(err)
	}
	if !models.HasModel(p.model) {
		return implemented.NewNoModelError(p.model, p.baseExtra())
	}
	return nil
}

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
