package openai

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
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

type OpenAI struct {
	apiKey      string
	provider    string
	baseURL     string
	baseURLFunc func(model string) string
	Log         *slog.Logger `json:"-"`
}

// CompatibleConfig configures an OpenAI-compatible backend (xAI, vLLM, Fireworks,
// oMLX, etc.) that speaks the /v1/responses (and optionally /v1/embeddings) API.
// Exactly one of BaseURL or BaseURLFunc should be set; if both are provided,
// BaseURLFunc wins.
type CompatibleConfig struct {
	Provider    string
	APIKey      string
	BaseURL     string
	BaseURLFunc func(model string) string
}

func New(key string) *OpenAI {
	return &OpenAI{
		apiKey:   key,
		provider: Provider,
	}
}

// NewCompatible builds a client for a provider that speaks the OpenAI API shape
// but lives at a different base URL. Wrappers like services/xai and services/vllm
// use this to declare their identity and endpoint without reimplementing the
// request/response pipeline.
func NewCompatible(cfg CompatibleConfig) *OpenAI {
	name := cfg.Provider
	if name == "" {
		name = Provider
	}
	return &OpenAI{
		apiKey:      cfg.APIKey,
		provider:    name,
		baseURL:     cfg.BaseURL,
		baseURLFunc: cfg.BaseURLFunc,
	}
}

func (g *OpenAI) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	args = append([]any{"provider", g.provider}, args...)
	g.Log.Debug("[bellman/"+g.provider+"] "+msg, args...)
}

func (g *OpenAI) Provider() string {
	return g.provider
}

func (g *OpenAI) Embed(request *embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)
	if len(request.Texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	reqModel := embedRequest{
		Input:          request.Texts,
		Model:          request.Model.Name,
		EncodingFormat: "float",
	}

	u, err := url.JoinPath(g.getBaseURL(request.Model.Name), "/v1/embeddings")
	if err != nil {
		return nil, fmt.Errorf("could not construct embeddings URL, %w", err)
	}

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal openai request, %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create openai request, %w", err)
	}
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)

	if err != nil {
		return nil, fmt.Errorf("could not post openai request, %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(os.Stdout, resp.Body)
		return nil, fmt.Errorf("unexpected status code, %d", resp.StatusCode)
	}

	var respModel embedResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)

	if err != nil {
		return nil, fmt.Errorf("could not decode openai response, %w", err)
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

func (g *OpenAI) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {
	return nil, fmt.Errorf("not supported by %s embed models", g.provider)
}

func (g *OpenAI) Generator(options ...gen.Option) *gen.Generator {
	var _gen = &gen.Generator{
		Prompter: &generator{
			openai: g,
		},
		Request: gen.Request{},
	}

	for _, op := range options {
		_gen = op(_gen)
	}

	return _gen
}

func (g *OpenAI) SetBaseURL(baseURL string) *OpenAI {
	g.baseURL = baseURL
	return g
}

func (g *OpenAI) SetBaseURLFunc(f func(model string) string) *OpenAI {
	g.baseURLFunc = f
	return g
}

func (g *OpenAI) getBaseURL(model string) string {
	if g.baseURLFunc != nil {
		return g.baseURLFunc(model)
	}
	if g.baseURL != "" {
		return g.baseURL
	}
	return "https://api.openai.com"
}

func (g *OpenAI) SetLogger(logger *slog.Logger) *OpenAI {
	g.Log = logger
	return g
}
