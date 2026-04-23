package omlx

import (
	"github.com/modfin/bellman/services/openai"
)

func New(baseURL string, apiKey string) *openai.OpenAI {
	return openai.NewCompatible(openai.CompatibleConfig{
		Provider: Provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	})
}
