package vllm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
)

type embedRequest struct {
	Input          []string `json:"input"`
	Model          string   `json:"model"`
	EncodingFormat string   `json:"encoding_format"`
}

type embedResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"Model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type VLLM struct {
	modelToUri map[string]string
	Log        *slog.Logger `json:"-"`
}

func New(uris []string, models []string) *VLLM {
	m := make(map[string]string)
	for i, model := range models {
		m[model] = uris[i]
	}
	return &VLLM{
		modelToUri: m,
	}
}

func (g *VLLM) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/open_ai] "+msg, args...)
}

func (g *VLLM) Provider() string {
	return Provider
}

func (g *VLLM) Embed(request *embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)
	if len(request.Texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	reqModel := embedRequest{
		Input:          request.Texts,
		Model:          request.Model.Name,
		EncodingFormat: "float",
	}
	uri := g.modelToUri[request.Model.Name]
	if uri == "" {
		return nil, fmt.Errorf("model %s not found", request.Model.Name)
	}

	u, err := url.JoinPath(uri, "/v1/embeddings")
	if err != nil {
		return nil, fmt.Errorf("could not join url, %w", err)
	}

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal vllm request, %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create vllm request, %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("could not post vllm request, %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(os.Stdout, resp.Body)
		return nil, fmt.Errorf("unexpected status code, %d", resp.StatusCode)
	}

	var respModel embedResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)

	if err != nil {
		return nil, fmt.Errorf("could not decode vllm response, %w", err)
	}
	if len(respModel.Data) == 0 {
		return nil, fmt.Errorf("no data in response")
	}
	if len(respModel.Data) != len(request.Texts) {
		return nil, fmt.Errorf("wrong number of embeddings, %d, expected %d", len(respModel.Data), len(request.Texts))
	}

	g.log("[embed] response", "request", reqc, "token-total", respModel.Usage.TotalTokens)
	embeddingResp := &embed.Response{
		Embeddings: make([][]float64, len(respModel.Data)),
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: respModel.Usage.TotalTokens,
		},
	}
	for idx, data := range respModel.Data {
		embeddingResp.Embeddings[idx] = data.Embedding
	}

	return embeddingResp, nil
}

func (g *VLLM) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by vllm embed models")
}

func (g *VLLM) Generator(options ...gen.Option) *gen.Generator {
	var gen = &gen.Generator{
		Prompter: &generator{
			vllm: g,
		},
		Request: gen.Request{},
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}

func (g *VLLM) SetLogger(logger *slog.Logger) *VLLM {
	g.Log = logger
	return g
}
