package embed

import (
	"github.com/modfin/bellman/models"
)

type Embeder interface {
	Embed(embed Request) (*Response, error)
}

type Model struct {
	Provider    string `json:"provider"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	InputMaxTokens   int `json:"input_max_tokens,omitempty"`
	OutputDimensions int `json:"output_dimensions,omitempty"`
}

func (m Model) FQN() string {
	return m.String()
}

func (m Model) String() string {
	return m.Provider + "/" + m.Name
}

type Request struct {
	Model Model `json:"model"`

	Text string `json:"text"`
}

type Response struct {
	Embedding []float64       `json:"embedding"`
	Metadata  models.Metadata `json:"metadata,omitempty"`
}
