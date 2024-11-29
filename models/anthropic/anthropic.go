package anthropic

import (
	"github.com/modfin/bellman"
)

type Anthropic struct {
	apiKey string
}

func New(apiKey string) *Anthropic {
	return &Anthropic{
		apiKey: apiKey,
	}
}

func (a *Anthropic) Generator(options ...bellman.GeneratorOption) bellman.Generator {
	var gen bellman.Generator = &generator{
		a: a,

		topP:        -1,
		temperature: 1,
		maxTokens:   1024,
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}
