package anthropic

import (
	"github.com/modfin/bellman/models/gen"
	"log/slog"
)

type Anthropic struct {
	apiKey string
	Log    *slog.Logger `json:"-"`
}

func New(apiKey string) *Anthropic {
	return &Anthropic{
		apiKey: apiKey,
	}
}

func (g *Anthropic) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/anthropic] "+msg, args...)
}
func (g *Anthropic) Provider() string {
	return Provider
}
func (a *Anthropic) Generator(options ...gen.Option) *gen.Generator {
	var gen = &gen.Generator{
		Prompter: &generator{
			anthropic: a,
		},
		Request: gen.Request{},
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}

func (g *Anthropic) SetLogger(logger *slog.Logger) *Anthropic {
	g.Log = logger
	return g

}
