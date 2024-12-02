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

func (a *Anthropic) Generator(options ...bellman.GeneratorOption) *bellman.Generator {
	var gen = &bellman.Generator{
		Prompter: &generator{
			anthropic: a,
		},
		Config: bellman.Config{
			TopP:        -1,
			Temperature: 1,
			MaxTokens:   1024,
		},
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}
