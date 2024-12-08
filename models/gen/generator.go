package gen

import (
	"errors"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type Generator struct {
	Prompter Prompter
	Request  Request
}

func (b *Generator) SetConfig(config Request) *Generator {
	bb := b.clone()
	bb.Request = config
	return bb
}

func (b *Generator) Prompt(prompts ...prompt.Prompt) (*Response, error) {
	prompter := b.Prompter
	if prompter == nil {
		return nil, errors.New("prompter is required")
	}
	prompter.SetRequest(b.clone().Request)
	return prompter.Prompt(prompts...)
}

func (b *Generator) clone() *Generator {
	var bb Generator
	bb = *b
	if b.Request.OutputSchema != nil {
		cp := *b.Request.OutputSchema
		bb.Request.OutputSchema = &cp
	}
	if b.Request.ToolConfig != nil {
		cp := *b.Request.ToolConfig
		bb.Request.ToolConfig = &cp
	}
	if b.Request.Tools != nil {
		bb.Request.Tools = append([]tools.Tool{}, b.Request.Tools...)
	}

	return &bb
}

func (b *Generator) Model(model Model) *Generator {
	bb := b.clone()
	bb.Request.Model = model
	return bb
}

func (b *Generator) System(prompt string) *Generator {
	bb := b.clone()
	bb.Request.SystemPrompt = prompt
	return bb
}

func (b *Generator) SetOutputSchema(element any) *Generator {
	bb := b.clone()
	bb.Request.OutputSchema = schema.New(element)
	return bb
}
func (g *Generator) Tools() []tools.Tool {
	return g.Request.Tools
}

func (b *Generator) SetTools(tool ...tools.Tool) *Generator {
	bb := b.clone()

	bb.Request.Tools = append([]tools.Tool{}, tool...)
	return bb
}
func (g *Generator) AddTools(tool ...tools.Tool) *Generator {
	return g.SetTools(append(g.Request.Tools, tool...)...)
}

func (b *Generator) SetToolConfig(tool tools.Tool) *Generator {
	bb := b.clone()
	bb.Request.ToolConfig = &tool

	for _, t := range tools.ControlTools {
		if t.Name == tool.Name {
			return bb
		}
	}
	bb.Request.Tools = []tools.Tool{tool}
	return bb
}

func (b *Generator) StopAt(stop ...string) *Generator {
	bb := b.clone()
	bb.Request.StopSequences = append([]string{}, stop...)

	return bb
}

func (b *Generator) Temperature(temperature float64) *Generator {
	bb := b.clone()
	bb.Request.Temperature = temperature

	return bb
}

func (b *Generator) TopP(topP float64) *Generator {
	bb := b.clone()
	bb.Request.TopP = topP

	return bb
}

func (b *Generator) MaxTokens(maxTokens int) *Generator {
	bb := b.clone()
	bb.Request.MaxTokens = maxTokens

	return bb
}

type Option func(generator *Generator) *Generator

func WithRequest(requset Request) Option {
	return func(g *Generator) *Generator {
		return g.SetConfig(requset)
	}
}

func WithModel(model Model) Option {
	return func(g *Generator) *Generator {
		return g.Model(model)
	}
}

func WithTools(tools ...tools.Tool) Option {
	return func(g *Generator) *Generator {
		return g.SetTools(tools...)
	}
}

func WithToolConfig(tool tools.Tool) Option {
	return func(g *Generator) *Generator {
		return g.SetToolConfig(tool)
	}
}

func WithSystem(prompt string) Option {
	return func(g *Generator) *Generator {
		return g.System(prompt)
	}
}

func WithOutput(element any) Option {
	return func(g *Generator) *Generator {
		return g.SetOutputSchema(element)
	}
}

func WithStopAt(stop ...string) Option {
	return func(g *Generator) *Generator {
		return g.StopAt(stop...)
	}
}

func WithTemperature(temperature float64) Option {
	return func(g *Generator) *Generator {
		return g.Temperature(temperature)
	}
}

func WithTopP(topP float64) Option {
	return func(g *Generator) *Generator {
		return g.TopP(topP)
	}
}

func WithMaxTokens(maxTokens int) Option {
	return func(g *Generator) *Generator {
		return g.MaxTokens(maxTokens)
	}
}
