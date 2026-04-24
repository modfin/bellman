package xai

import (
	"github.com/modfin/bellman/services/openai"
)

const baseURL = "https://api.x.ai"

func New(apiKey string) *openai.OpenAI {
	return openai.NewCompatible(openai.CompatibleConfig{
		Provider: Provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	})
}
