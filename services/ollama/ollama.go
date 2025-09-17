package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
)

type embedRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}
type embedResponse struct {
	Embedding [][]float64 `json:"embeddings"`

	TotalDuration   int `json:"total_duration"`
	LoadDuration    int `json:"load_duration"`
	PromptEvalCount int `json:"prompt_eval_count"`
}

type Ollama struct {
	uri string
	Log *slog.Logger `json:"-"`
}

func New(uri string) *Ollama {
	return &Ollama{
		uri: uri,
	}
}

func (g *Ollama) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/ollama] "+msg, args...)
}

func (g *Ollama) Provider() string {
	return Provider
}

func (g *Ollama) Embed(request *embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)
	if len(request.Texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	var embeddings [][]float64
	var tokenTotal int
	for _, text := range request.Texts {
		reqModel := embedRequest{
			Input: text,
			Model: request.Model.Name,
		}

		body, err := json.Marshal(reqModel)
		if err != nil {
			return nil, fmt.Errorf("could not marshal openai request, %w", err)
		}

		u, err := url.JoinPath(g.uri, "/api/embed")
		if err != nil {
			return nil, fmt.Errorf("could not join url, %w", err)
		}
		req, err := http.NewRequest("POST", u, bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("could not create openai request, %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)

		if err != nil {
			return nil, fmt.Errorf("could not post openai request, %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			d, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("unexpected status code, %d, %s", resp.StatusCode, string(d))
		}

		var respModel embedResponse
		err = json.NewDecoder(resp.Body).Decode(&respModel)

		if err != nil {
			return nil, fmt.Errorf("could not decode openai response, %w", err)
		}

		if len(respModel.Embedding) == 0 {
			return nil, fmt.Errorf("no embeddings in response")
		}
		embeddings = append(embeddings, respModel.Embedding[0])
		tokenTotal += respModel.PromptEvalCount

	}

	g.log("[embed] response", "request", reqc, "token-total", tokenTotal)

	return &embed.Response{
		Embeddings: embeddings,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: tokenTotal,
		},
	}, nil
}

func (g *Ollama) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by ollama embed models")
}

func (g *Ollama) Generator(options ...gen.Option) *gen.Generator {
	var gen = &gen.Generator{
		Prompter: &generator{
			ollama: g,
		},
		Request: gen.Request{},
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}

func (g *Ollama) SetLogger(logger *slog.Logger) *Ollama {
	g.Log = logger
	return g
}
