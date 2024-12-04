package bellman

import "errors"

var ErrModelNotFound = errors.New("model not found")

type Proxy struct {
	embeders map[string]Embeder
	llms     map[string]LLM
}

func NewProxy() *Proxy {
	p := &Proxy{
		embeders: map[string]Embeder{},
		llms:     map[string]LLM{},
	}

	return p
}

func (p *Proxy) RegisterEmbeder(embeder Embeder, models ...EmbedModel) {
	for _, model := range models {
		p.embeders[model.Name] = embeder
	}
}
func (p *Proxy) RegisterLLM(llm LLM, models ...GenModel) {
	for _, model := range models {
		p.llms[model.Name] = llm
	}
}

func (p *Proxy) Embed(text string, model EmbedModel) ([]float64, error) {
	embeder, ok := p.embeders[model.Name]
	if !ok {
		return nil, ErrModelNotFound
	}

	return embeder.Embed(text, model)
}

func (p *Proxy) LLM(model GenModel) (LLM, error) {
	llm, ok := p.llms[model.Name]
	if !ok {
		return nil, ErrModelNotFound
	}

	return llm, nil
}
