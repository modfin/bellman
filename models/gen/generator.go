package gen

import (
	"context"
	"errors"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

type Generator struct {
	Prompter Prompter
	Request  Request
}

func Float(f float64) *float64 {
	return &f
}
func Int(i int) *int {
	return &i
}
func (b *Generator) SetConfig(config Request) *Generator {
	bb := b.clone()
	bb.Request = config
	return bb
}

func (b *Generator) Stream(prompts ...prompt.Prompt) (<-chan *StreamResponse, error) {
	prompter := b.Prompter
	if prompter == nil {
		return nil, errors.New("prompter is required")
	}
	r := b.clone().Request
	r.Stream = true
	prompter.SetRequest(r)
	return prompter.Stream(prompts...)
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
	if b.Request.WebSearchTool != nil {
		cp := *b.Request.WebSearchTool
		bb.Request.WebSearchTool = &cp
	}
	if b.Request.PresencePenalty != nil {
		cp := *b.Request.PresencePenalty
		bb.Request.PresencePenalty = &cp
	}
	if b.Request.FrequencyPenalty != nil {
		cp := *b.Request.FrequencyPenalty
		bb.Request.FrequencyPenalty = &cp
	}
	if b.Request.Temperature != nil {
		cp := *b.Request.Temperature
		bb.Request.Temperature = &cp
	}
	if b.Request.TopP != nil {
		cp := *b.Request.TopP
		bb.Request.TopP = &cp
	}
	if b.Request.TopK != nil {
		cp := *b.Request.TopK
		bb.Request.TopK = &cp
	}
	if b.Request.MaxTokens != nil {
		cp := *b.Request.MaxTokens
		bb.Request.MaxTokens = &cp
	}
	if b.Request.Context != nil {
		bb.Request.Context = b.Request.Context
	}
	if b.Request.ThinkingBudget != nil {
		cp := *b.Request.ThinkingBudget
		bb.Request.ThinkingBudget = &cp
	}
	if b.Request.ThinkingParts != nil {
		cp := *b.Request.ThinkingParts
		bb.Request.ThinkingParts = &cp
	}
	if b.Request.StopSequences != nil {
		bb.Request.StopSequences = append([]string{}, b.Request.StopSequences...)
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

func (b *Generator) Output(s *schema.JSON) *Generator {
	bb := b.clone()
	bb.Request.OutputSchema = s
	return bb
}
func (b *Generator) StrictOutput(strict bool) *Generator {
	bb := b.clone()
	bb.Request.StrictOutput = strict
	return bb
}
func (b *Generator) Tools() []tools.Tool {
	return b.Request.Tools
}

func (b *Generator) SetTools(tool ...tools.Tool) *Generator {
	bb := b.clone()

	bb.Request.Tools = append([]tools.Tool{}, tool...)
	return bb
}
func (b *Generator) AddTools(tool ...tools.Tool) *Generator {
	return b.SetTools(append(b.Request.Tools, tool...)...)
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
func (b *Generator) SetWebSearchTool(webSearchTool tools.WebSearchTool) *Generator {
	bb := b.clone()

	bb.Request.WebSearchTool = &webSearchTool
	return bb
}

func (b *Generator) StopAt(stop ...string) *Generator {
	bb := b.clone()
	bb.Request.StopSequences = append([]string{}, stop...)

	return bb
}

func (b *Generator) Temperature(temperature float64) *Generator {
	bb := b.clone()
	bb.Request.Temperature = &temperature

	return bb
}
func (b *Generator) FrequencyPenalty(freq float64) *Generator {
	bb := b.clone()
	bb.Request.FrequencyPenalty = &freq

	return bb
}
func (b *Generator) PresencePenalty(prec float64) *Generator {
	bb := b.clone()
	bb.Request.PresencePenalty = &prec

	return bb
}

func (b *Generator) TopP(topP float64) *Generator {
	bb := b.clone()
	bb.Request.TopP = &topP

	return bb
}
func (b *Generator) TopK(topK int) *Generator {
	bb := b.clone()
	bb.Request.TopK = &topK

	return bb
}
func (b *Generator) WithContext(ctx context.Context) *Generator {
	bb := b.clone()
	bb.Request.Context = ctx

	return bb
}

func (b *Generator) MaxTokens(maxTokens int) *Generator {
	bb := b.clone()
	bb.Request.MaxTokens = &maxTokens

	return bb
}

// ThinkingBudget sets the thinking budget for the generator. For models which do not support tokens as thinking budget,
// the number of tokens is translated into enums "low", "medium", "high". Where "low" is <2.000, "medium" is 2.000-10.000, and "high" is 10.001+.
func (b *Generator) ThinkingBudget(thinkingBudget int) *Generator {
	bb := b.clone()
	bb.Request.ThinkingBudget = &thinkingBudget

	return bb
}
func (b *Generator) IncludeThinkingParts(thinkingParts bool) *Generator {
	bb := b.clone()
	bb.Request.ThinkingParts = &thinkingParts

	return bb
}

type Option func(generator *Generator) *Generator

func WithRequest(req Request) Option {
	return func(g *Generator) *Generator {
		return g.SetConfig(req)
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

func WithOutput(s *schema.JSON) Option {
	return func(g *Generator) *Generator {
		return g.Output(s)
	}
}
func WithStrictOutput(strict bool) Option {
	return func(g *Generator) *Generator {
		return g.StrictOutput(strict)
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

func WithPresencePenalty(presence float64) Option {
	return func(g *Generator) *Generator {
		return g.PresencePenalty(presence)
	}
}
func WithFrequencyPenalty(freq float64) Option {
	return func(g *Generator) *Generator {
		return g.FrequencyPenalty(freq)
	}
}

func WithTopP(topP float64) Option {
	return func(g *Generator) *Generator {
		return g.TopP(topP)
	}
}
func WithTopK(topK int) Option {
	return func(g *Generator) *Generator {
		return g.TopK(topK)
	}
}

func WithMaxTokens(maxTokens int) Option {
	return func(g *Generator) *Generator {
		return g.MaxTokens(maxTokens)
	}
}
func WithContext(ctx context.Context) Option {
	return func(g *Generator) *Generator {
		return g.WithContext(ctx)
	}
}
func WithThinkingBudget(thinkingBudget int) Option {
	return func(g *Generator) *Generator {
		return g.ThinkingBudget(thinkingBudget)
	}
}
func WithThinkingParts(thinkingParts bool) Option {
	return func(g *Generator) *Generator {
		return g.IncludeThinkingParts(thinkingParts)
	}
}
