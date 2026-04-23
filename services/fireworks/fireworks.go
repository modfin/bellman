package fireworks

import (
	"github.com/modfin/bellman/services/openai"
)

const baseURL = "https://api.fireworks.ai/inference"

func New(apiKey string) *openai.OpenAI {
	return openai.NewCompatible(openai.CompatibleConfig{
		Provider: Provider,
		APIKey:   apiKey,
		BaseURL:  baseURL,
	})
}
