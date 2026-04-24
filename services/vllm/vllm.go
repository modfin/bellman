package vllm

import (
	"github.com/modfin/bellman/services/openai"
)

func New(uris []string, models []string) *openai.OpenAI {
	m := make(map[string]string, len(models))
	for i, model := range models {
		m[model] = uris[i]
	}
	return openai.NewCompatible(openai.CompatibleConfig{
		Provider: Provider,
		BaseURLFunc: func(model string) string {
			if u, ok := m[model]; ok {
				return u
			}
			return m["*"]
		},
	})
}
