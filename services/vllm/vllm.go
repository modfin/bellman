package vllm

import (
	"fmt"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/services/openai"
)

type VLLM struct {
	*openai.OpenAI
}

func New(uris []string, models []string) *VLLM {
	m := make(map[string]string)
	for i, model := range models {
		m[model] = uris[i]
	}
	v := &VLLM{}
	v.OpenAI = openai.New("").SetBaseURLFunc(func(model string) string {
		if u, ok := m[model]; ok {
			return u
		}
		return m["*"]
	})
	return v
}

func (v *VLLM) Provider() string {
	return Provider
}

func (v *VLLM) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by vllm embed models")
}
