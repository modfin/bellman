package bellman

import (
	"errors"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"maps"
	"slices"
)

var ErrModelNotFound = errors.New("model not found")

type Proxy struct {
	embedModels map[string]embed.Model
	genModels   map[string]gen.Model

	embeders map[string]embed.Embeder
	gens     map[string]gen.Gen
}

func NewProxy() *Proxy {
	p := &Proxy{
		embedModels: map[string]embed.Model{},
		embeders:    map[string]embed.Embeder{},
		genModels:   map[string]gen.Model{},
		gens:        map[string]gen.Gen{},
	}

	return p
}

func (p *Proxy) RegisterEmbeder(embeder embed.Embeder, models ...embed.Model) {
	for _, model := range models {
		p.embeders[model.String()] = embeder
		p.embedModels[model.String()] = model
	}
}
func (p *Proxy) RegisterGen(llm gen.Gen, models ...gen.Model) {
	for _, model := range models {
		p.gens[model.String()] = llm
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
	_, ok := p.embeders[model.FQN()]
	if ok {
		return true
	}
	_, ok = p.gens[model.FQN()]
	return ok
}

func (p *Proxy) Embed(embed embed.Request) (*embed.Response, error) {
	embeder, ok := p.embeders[embed.Model.String()]
	if !ok {
		return nil, ErrModelNotFound
	}

	return embeder.Embed(embed)
}

func (p *Proxy) Gen(model gen.Model) (*gen.Generator, error) {
	llm, ok := p.gens[model.String()]
	if !ok {
		return nil, ErrModelNotFound
	}

	return llm.Generator(gen.WithModel(model)), nil
}
