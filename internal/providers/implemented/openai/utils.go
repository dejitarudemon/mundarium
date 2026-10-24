package openai

import (
	"encoding/json"
	"fmt"

	"github.com/dejitarudemon/mundarium/internal/tools/definition"
	"github.com/openai/openai-go/v3"
)

func endpoint(base, sep string, parts ...string) string {
	url := base

	for _, part := range parts {
		url += sep + part
	}

	return url
}

func convertTools(defs []definition.Definition) ([]openai.ChatCompletionToolUnionParam, error) {
	result := make([]openai.ChatCompletionToolUnionParam, 0, len(defs))

	for _, def := range defs {
		var parameters openai.FunctionParameters

		if err := json.Unmarshal(def.Parameters, &parameters); err != nil {
			return nil, fmt.Errorf(
				"tool %q: invalid parameters: %w",
				def.Name,
				err,
			)
		}

		result = append(result,
			openai.ChatCompletionFunctionTool(
				openai.FunctionDefinitionParam{
					Name:        def.Name,
					Description: openai.String(def.Description),
					Parameters:  parameters,
				},
			),
		)
	}

	return result, nil
}
