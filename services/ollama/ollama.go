package ollama

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
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
	uris      []string
	available chan int // Pool of available instance indices
	Log       *slog.Logger `json:"-"`
}

func New(uris ...string) *Ollama {
	if len(uris) == 0 {
		panic("at least one Ollama URI must be provided")
	}

	// Create a buffered channel with capacity equal to number of instances
	// Each instance index is added to represent an available instance
	available := make(chan int, len(uris))
	for i := range uris {
		available <- i
	}

	return &Ollama{
		uris:      uris,
		available: available,
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

	// Acquire an available instance (blocks if all instances are busy)
	instanceIdx := <-g.available
	defer func() {
		// Return the instance to the pool when done
		g.available <- instanceIdx
	}()

	uri := g.uris[instanceIdx]
	g.log("[embed] using instance", "index", instanceIdx, "uri", uri, "request", reqc)

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

		u, err := url.JoinPath(uri, "/api/embed")
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

		if resp.StatusCode != http.StatusOK {
			d, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("unexpected status code, %d, %s", resp.StatusCode, string(d))
		}

		var respModel embedResponse
		err = json.NewDecoder(resp.Body).Decode(&respModel)
		resp.Body.Close()

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