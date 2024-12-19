package bellman

import (
	"errors"
	"fmt"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

var ErrNoModelProvided = errors.New("no model was provided")
var ErrClientNotFound = errors.New("client not found")

type Proxy struct {
	embeders map[string]embed.Embeder
	gens     map[string]gen.Gen
}

func NewProxy() *Proxy {
	p := &Proxy{
		embeders: map[string]embed.Embeder{},
		gens:     map[string]gen.Gen{},
	}

	return p
}

func (p *Proxy) RegisterEmbeder(embeder embed.Embeder) {
	p.embeders[embeder.Provider()] = embeder
}
func (p *Proxy) RegisterGen(llm gen.Gen) {
	p.gens[llm.Provider()] = llm
}

func (p *Proxy) Embed(embed embed.Request) (*embed.Response, error) {
	client, ok := p.embeders[embed.Model.Provider]
	if !ok {
		return nil, fmt.Errorf("no client registerd for provider '%s', %w", embed.Model.Provider, ErrClientNotFound)
	}

	if client == nil {
		return nil, ErrNoModelProvided
	}

	if embed.Model.Name == "" {
		return nil, fmt.Errorf("embed.Model.Name is not set, %w", ErrNoModelProvided)
	}
	return client.Embed(embed)
}

func (p *Proxy) Gen(model gen.Model) (*gen.Generator, error) {
	client, ok := p.gens[model.Provider]
	if !ok {
		return nil, fmt.Errorf("no client registerd for provider '%s', %w", model.Provider, ErrClientNotFound)
	}

	if client == nil {
		return nil, ErrClientNotFound
	}

	if model.Name == "" {
		return nil, fmt.Errorf("model.Name is not set, %w", ErrNoModelProvided)
	}

	return client.Generator(gen.WithModel(model)), nil
}
