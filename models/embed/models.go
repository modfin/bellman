package embed

import (
	"context"
	"github.com/modfin/bellman/models"
)

type Embeder interface {
	Provider() string
	Embed(embed Request) (*Response, error)
}

type Mode string

const ModeQuery = "query"
const ModeDocument = "document"
const ModeNone = ""

type Model struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`

	Mode Mode `json:"mode,omitempty"`

	Description string `json:"description,omitempty"`

	InputMaxTokens   int `json:"input_max_tokens,omitempty"`
	OutputDimensions int `json:"output_dimensions,omitempty"`
}

func (m Model) WithMode(mode Mode) Model {
	m.Mode = mode
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
