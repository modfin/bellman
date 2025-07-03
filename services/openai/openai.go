package openai

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
	"os"
	"sync/atomic"
)

type embedRequest struct {
	Input          string `json:"input"`
	Model          string `json:"Model"`
	EncodingFormat string `json:"encoding_format"`
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

//type Request struct {
//	GenModel   string `cli:"ai-openai-gen-Model"`
//	EmbedModel string `cli:"ai-openai-embedding-Model"`
//	ApiKey string `cli:"ai-openai-api-key"`
//}

type OpenAI struct {
	apiKey string
	Log    *slog.Logger `json:"-"`
}

func New(key string) *OpenAI {
	return &OpenAI{
		apiKey: key,
	}
}

func (g *OpenAI) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/open_ai] "+msg, args...)
}

func (g *OpenAI) Provider() string {
	return Provider
}
func (g *OpenAI) Embed(request embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)

	reqModel := embedRequest{
		Input:          request.Text,
		Model:          request.Model.Name,
		EncodingFormat: "float",
	}

	u := "https://api.openai.com/v1/embeddings"

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal openai request, %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create openai request, %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+g.apiKey)
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

	g.log("[embed] response", "request", reqc, "token-total", respModel.Usage.TotalTokens)

	return &embed.Response{
		Embedding: respModel.Data[0].Embedding,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: respModel.Usage.TotalTokens,
		},
	}, nil
}

func (g *OpenAI) Generator(options ...gen.Option) *gen.Generator {
	var gen_ = &gen.Generator{
		Prompter: &generator{
			openai: g,
		},
		Request: gen.Request{},
	}

	for _, op := range options {
		gen_ = op(gen_)
	}

	return gen_
}

func (g *OpenAI) SetLogger(logger *slog.Logger) *OpenAI {
	g.Log = logger
	return g
}
