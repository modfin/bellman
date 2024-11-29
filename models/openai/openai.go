package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"io"
	"net/http"
	"os"
)

type embedRequest struct {
	Input          string `json:"input"`
	Model          string `json:"model"`
	EncodingFormat string `json:"encoding_format"`
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

//type Config struct {
//	GenModel   string `cli:"ai-openai-gen-model"`
//	EmbedModel string `cli:"ai-openai-embedding-model"`
//	ApiKey string `cli:"ai-openai-api-key"`
//}

type OpenAI struct {
	apiKey string
}

func New(key string) *OpenAI {
	return &OpenAI{
		apiKey: key,
	}
}

func (g *OpenAI) Embed(text string, model string) ([]float64, error) {
	reqModel := embedRequest{
		Input:          text,
		Model:          model,
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

	return respModel.Data[0].Embedding, nil
}

func (g *OpenAI) Generate(options ...bellman.GeneratorOption) bellman.Generator {
	var gen bellman.Generator = &generator{
		g: g,

		topP:        1,
		temperature: 1,
		maxTokens:   2048,
	}

	for _, op := range options {
		gen = op(gen)
	}

	return gen
}