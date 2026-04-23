package omlx

import (
	"fmt"

	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/services/openai"
)

type OMLX struct {
	*openai.OpenAI
}

func New(baseUrl string, apiKey string) *OMLX {
	o := &OMLX{}
	o.OpenAI = openai.New(apiKey).SetBaseURL(baseUrl)
	return o
}

func (o *OMLX) Provider() string {
	return Provider
}

func (o *OMLX) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by omlx embed models")
}
