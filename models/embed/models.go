package embed

import (
	"context"
	"errors"
	"github.com/modfin/bellman/models"
	"strings"
)

type Embeder interface {
	Provider() string
	Embed(embed Request) (*Response, error)
	EmbedMany(embed RequestMany) (*ResponseMany, error)
	EmbedDocument(embed RequestDocument) (*ResponseDocument, error)
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

	Config map[string]any `json:"config,omitempty"`
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
func ToModel(fqn string) (Model, error) {
	provider, name, found := strings.Cut(fqn, "/")
	if !found {
		return Model{}, errors.New("invalid fqn, did not find a '/' seperating provider and model")
	}
	return Model{
		Provider: provider,
		Name:     name,
	}, nil
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

type RequestMany struct {
	Ctx   context.Context `json:"-"`
	Model Model           `json:"model"`
	Texts []string        `json:"texts"`
}

type ResponseMany struct {
	Embeddings [][]float64     `json:"embeddings"`
	Metadata   models.Metadata `json:"metadata,omitempty"`
}

type RequestDocument struct {
	Ctx            context.Context `json:"-"`
	Model          Model           `json:"model"`
	DocumentChunks []string        `json:"document_chunks"`
}

type ResponseDocument struct {
	Embeddings [][]float64     `json:"embeddings"`
	Metadata   models.Metadata `json:"metadata,omitempty"`
}
