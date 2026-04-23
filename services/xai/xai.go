package xai

import (
	"fmt"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/services/openai"
)

const baseURL = "https://api.x.ai"

type XAI struct {
	*openai.OpenAI
}

func New(apiKey string) *XAI {
	x := &XAI{}
	x.OpenAI = openai.New(apiKey).SetBaseURL(baseURL)
	return x
}

func (x *XAI) Provider() string {
	return Provider
}

func (x *XAI) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by xai embed models")
}
