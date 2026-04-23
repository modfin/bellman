package fireworks

import (
	"fmt"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/services/openai"
)

const baseURL = "https://api.fireworks.ai/inference"

type Fireworks struct {
	*openai.OpenAI
}

func New(apiKey string) *Fireworks {
	f := &Fireworks{}
	f.OpenAI = openai.New(apiKey).SetBaseURL(baseURL)
	return f
}

func (f *Fireworks) Provider() string {
	return Provider
}

func (f *Fireworks) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by fireworks embed models")
}
