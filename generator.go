package bellman

import (
	"errors"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
	"log/slog"
)

type Config struct {
	Model        GenModel
	SystemPrompt string `json:"system_prompt"`

	StopSequences []string `json:"stop_sequences"`
	TopP          float64  `json:"top_p"`
	Temperature   float64  `json:"temperature"`
	MaxTokens     int      `json:"max_tokens"`

	OutputSchema *schema.JSON `json:"output_schema"`

	Tools      []tools.Tool `json:"tools"`
	ToolConfig *tools.Tool  `json:"tool"`

	Log *slog.Logger `json:"-"`
}

type Generator struct {
	Prompter Prompter
	Config   Config
}

func (b *Generator) SetConfig(config Config) *Generator {
	bb := b.clone()
	bb.Config = config
	return bb
}

func (b *Generator) Prompt(prompts ...prompt.Prompt) (Response, error) {
	prompter := b.Prompter
	if prompter == nil {
		return nil, errors.New("prompter is required")
	}
	prompter.SetConfig(b.clone().Config)
	return prompter.Prompt(prompts...)
}

func (b *Generator) clone() *Generator {
	var bb Generator
	bb = *b
	if b.Config.OutputSchema != nil {
		cp := *b.Config.OutputSchema
		bb.Config.OutputSchema = &cp
	}
	if b.Config.ToolConfig != nil {
		cp := *b.Config.ToolConfig
		bb.Config.ToolConfig = &cp
	}
	if b.Config.Tools != nil {
		bb.Config.Tools = append([]tools.Tool{}, b.Config.Tools...)
	}

	return &bb
}

func (b *Generator) SetLogger(log *slog.Logger) *Generator {
	bb := b.clone()
	bb.Config.Log = log
	return bb
}

func (b *Generator) Model(model GenModel) *Generator {
	bb := b.clone()
	bb.Config.Model = model
	return bb
}

func (b *Generator) System(prompt string) *Generator {
	bb := b.clone()
	bb.Config.SystemPrompt = prompt
	return bb
}

func (b *Generator) SetOutputSchema(element any) *Generator {
	bb := b.clone()
	bb.Config.OutputSchema = schema.New(element)
	return bb
}
func (g *Generator) Tools() []tools.Tool {
	return g.Config.Tools
}

func (b *Generator) SetTools(tool ...tools.Tool) *Generator {
	bb := b.clone()

	bb.Config.Tools = append([]tools.Tool{}, tool...)
	return bb
}
func (g *Generator) AddTools(tool ...tools.Tool) *Generator {
	return g.SetTools(append(g.Config.Tools, tool...)...)
}

func (b *Generator) SetToolConfig(tool tools.Tool) *Generator {
	bb := b.clone()
	bb.Config.ToolConfig = &tool

	for _, t := range tools.ControlTools {
		if t.Name == tool.Name {
			return bb
		}
	}
	bb.Config.Tools = []tools.Tool{tool}
	return bb
}

func (b *Generator) StopAt(stop ...string) *Generator {
	bb := b.clone()
	bb.Config.StopSequences = append([]string{}, stop...)

	return bb
}

func (b *Generator) Temperature(temperature float64) *Generator {
	bb := b.clone()
	bb.Config.Temperature = temperature

	return bb
}

func (b *Generator) TopP(topP float64) *Generator {
	bb := b.clone()
	bb.Config.TopP = topP

	return bb
}

func (b *Generator) MaxTokens(maxTokens int) *Generator {
	bb := b.clone()
	bb.Config.MaxTokens = maxTokens

	return bb
}

type GeneratorOption func(generator *Generator) *Generator

func WithConfig(config Config) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.SetConfig(config)
	}
}

func WithModel(model GenModel) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.Model(model)
	}
}

func WithTools(tools ...tools.Tool) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.SetTools(tools...)
	}
}

func WithToolConfig(tool tools.Tool) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.SetToolConfig(tool)
	}
}

func WithSystem(prompt string) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.System(prompt)
	}
}

func WithOutput(element any) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.SetOutputSchema(element)
	}
}

func WithStopAt(stop ...string) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.StopAt(stop...)
	}
}

func WithTemperature(temperature float64) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.Temperature(temperature)
	}
}

func WithTopP(topP float64) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.TopP(topP)
	}
}

func WithMaxTokens(maxTokens int) GeneratorOption {
	return func(g *Generator) *Generator {
		return g.MaxTokens(maxTokens)
	}
}
