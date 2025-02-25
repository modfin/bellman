package embed

import (
	"context"
	"github.com/modfin/bellman/models"
)

type Embeder interface {
	Provider() string
	Embed(embed Request) (*Response, error)
}

type Type string

const TypeQuery = "query"
const TypeDocument = "document"
const TypeNone = ""

type Model struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`

	Type Type `json:"type,omitempty"`

	Description string `json:"description,omitempty"`

	InputMaxTokens   int `json:"input_max_tokens,omitempty"`
	OutputDimensions int `json:"output_dimensions,omitempty"`
}

func (m Model) WithType(mode Type) Model {
	m.Type = mode
	return m
}

func (m Model) FQN() string {
	return m.String()
}

func (m Model) String() string {
	return m.Provider + "/" + m.Name
}

type Request struct {
	Ctx context.Context `json:"-"`

	Model Model `json:"model"`

	Text string `json:"text"`
}

type Response struct {
	Embedding []float64       `json:"embedding"`
	Metadata  models.Metadata `json:"metadata,omitempty"`
}

func (r *Response) AsFloat64() []float64 {
	output := make([]float64, len(r.Embedding))
	copy(output, r.Embedding)
	return output
}

func (r *Response) AsFloat32() []float32 {
	output := make([]float32, len(r.Embedding))
	for i, v := range r.Embedding {
		output[i] = float32(v)
	}
	return output
}
