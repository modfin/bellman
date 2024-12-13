package bellman

import (
	"errors"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"maps"
	"slices"
)

var ErrModelNotFound = errors.New("model not found")
var ErrClientNotFound = errors.New("client not found")

type Proxy struct {
	embedModels map[string]embed.Model
	genModels   map[string]gen.Model

	embedersByFQN map[string]embed.Embeder
	gensByFQN     map[string]gen.Gen
}

func NewProxy() *Proxy {
	p := &Proxy{
		embedModels:   map[string]embed.Model{},
		embedersByFQN: map[string]embed.Embeder{},
		genModels:     map[string]gen.Model{},
		gensByFQN:     map[string]gen.Gen{},
	}

	return p
}

func (p *Proxy) RegisterEmbeder(embeder embed.Embeder, models ...embed.Model) {
	for _, model := range models {
		p.embedersByFQN[model.String()] = embeder
		p.embedModels[model.String()] = model
	}
}
func (p *Proxy) RegisterGen(llm gen.Gen, models ...gen.Model) {
	for _, model := range models {
		p.gensByFQN[model.String()] = llm
		p.genModels[model.String()] = model
	}
}

func (p *Proxy) EmbedModels() []embed.Model {
	models := slices.Collect(maps.Values(p.embedModels))
	slices.SortFunc(models, func(i, j embed.Model) int {
		if i.String() < j.String() {
			return -1
		}
		return 1
	})
	return models
}

func (p *Proxy) GenModels() []gen.Model {
	models := slices.Collect(maps.Values(p.genModels))
	slices.SortFunc(models, func(i, j gen.Model) int {
		if i.String() < j.String() {
			return -1
		}
		return 1
	})
	return models
}

type Named interface {
	FQN() string
}

func (p *Proxy) HasModel(model Named) bool {
	_, ok := p.embedersByFQN[model.FQN()]
	if ok {
		return true
	}
	_, ok = p.gensByFQN[model.FQN()]
	return ok
}

func (p *Proxy) Embed(embed embed.Request, allowUnknown bool) (*embed.Response, error) {
	client := p.embedersByFQN[embed.Model.String()]

	if client == nil && allowUnknown {
		for _, m := range p.embedModels {
			if m.Provider == embed.Model.Provider {
				client = p.embedersByFQN[m.String()]
				break
			}
		}
	}

	if client == nil {
		return nil, ErrModelNotFound
	}

	return client.Embed(embed)
}

func (p *Proxy) Gen(model gen.Model, allowUnknown bool) (*gen.Generator, error) {
	client := p.gensByFQN[model.String()]

	if client == nil && allowUnknown {
		for _, m := range p.genModels {
			if m.Provider == model.Provider {
				client = p.gensByFQN[m.String()]
				break
			}
		}
	}

	if client == nil {
		return nil, ErrModelNotFound
	}

	return client.Generator(gen.WithModel(model)), nil
}
