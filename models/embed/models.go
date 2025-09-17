package embed

import (
	"context"
	"errors"
	"github.com/modfin/bellman/models"
	"strings"
)

type Embeder interface {
	Provider() string
	Embed(req *Request) (*Response, error)
	EmbedDocument(req *DocumentRequest) (*DocumentResponse, error)
}

type Type string

const (
	TypeQuery    Type = "query"
	TypeDocument Type = "document"
	TypeNone     Type = ""
)

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
		return Model{}, errors.New("invalid fqn, did not find a '/' separating provider and model")
	}
	return Model{
		Provider: provider,
		Name:     name,
	}, nil
}

type Request struct {
	Ctx   context.Context `json:"-"`
	Model Model           `json:"model"`
	Texts []string        `json:"texts"`
}

func NewSingleRequest(ctx context.Context, model Model, text string) *Request {
	return &Request{
		Ctx:   ctx,
		Model: model,
		Texts: []string{text},
	}
}

func NewManyRequest(ctx context.Context, model Model, texts []string) *Request {
	return &Request{
		Ctx:   ctx,
		Model: model,
		Texts: texts,
	}
}

func (r *Request) IsSingle() bool {
	return len(r.Texts) == 1
}

func (r *Request) FirstText() string {
	if len(r.Texts) > 0 {
		return r.Texts[0]
	}
	return ""
}

type Response struct {
	Embeddings [][]float64     `json:"embeddings"`
	Metadata   models.Metadata `json:"metadata,omitempty"`
}

func (r *Response) Single() ([]float64, error) {
	if len(r.Embeddings) != 1 {
		return nil, errors.New("response contains multiple embeddings, expected single")
	}
	return r.Embeddings[0], nil
}

func (r *Response) Many() [][]float64 {
	return r.Embeddings
}

func (r *Response) AsFloat64() [][]float64 {
	output := make([][]float64, len(r.Embeddings))
	for i, emb := range r.Embeddings {
		output[i] = make([]float64, len(emb))
		copy(output[i], emb)
	}
	return output
}

func (r *Response) AsFloat32() [][]float32 {
	output := make([][]float32, len(r.Embeddings))
	for i, emb := range r.Embeddings {
		output[i] = make([]float32, len(emb))
		for j, v := range emb {
			output[i][j] = float32(v)
		}
	}
	return output
}

func (r *Response) SingleAsFloat64() ([]float64, error) {
	emb, err := r.Single()
	if err != nil {
		return nil, err
	}
	output := make([]float64, len(emb))
	copy(output, emb)
	return output, nil
}

func (r *Response) SingleAsFloat32() ([]float32, error) {
	emb, err := r.Single()
	if err != nil {
		return nil, err
	}
	output := make([]float32, len(emb))
	for i, v := range emb {
		output[i] = float32(v)
	}
	return output, nil
}

type DocumentRequest struct {
	Ctx            context.Context `json:"-"`
	Model          Model           `json:"model"`
	DocumentChunks []string        `json:"document_chunks"`
}

func NewDocumentRequest(ctx context.Context, model Model, chunks []string) *DocumentRequest {
	return &DocumentRequest{
		Ctx:            ctx,
		Model:          model,
		DocumentChunks: chunks,
	}
}

type DocumentResponse struct {
	Embeddings [][]float64     `json:"embeddings"`
	Metadata   models.Metadata `json:"metadata,omitempty"`
}

func (r *DocumentResponse) AsFloat64() [][]float64 {
	output := make([][]float64, len(r.Embeddings))
	for i, emb := range r.Embeddings {
		output[i] = make([]float64, len(emb))
		copy(output[i], emb)
	}
	return output
}

func (r *DocumentResponse) AsFloat32() [][]float32 {
	output := make([][]float32, len(r.Embeddings))
	for i, emb := range r.Embeddings {
		output[i] = make([]float32, len(emb))
		for j, v := range emb {
			output[i][j] = float32(v)
		}
	}
	return output
}
