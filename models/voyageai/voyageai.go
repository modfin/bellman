package voyageai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Config struct {
	ApiKey         string `cli:"ai-voyage-api-key"`
	EmbeddingModel string `cli:"ai-voyage-embedding-model"`
}

type VoyageAI struct {
	config Config
}

func New(config Config) (*VoyageAI, error) {
	return &VoyageAI{config: config}, nil
}

type request struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type response struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Embedding []float64 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}

func (v *VoyageAI) Embed(text string) ([]float64, error) {

	model, ok := Models[v.config.EmbeddingModel]
	if !ok {
		return nil, fmt.Errorf("unknown model %s", v.config.EmbeddingModel)
	}

	u := `https://api.voyageai.com/v1/embeddings`

	reqModel := request{
		Input: []string{text},
		Model: model.Name,
	}
	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal request, %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(jsonReq))
	if err != nil {
		return nil, fmt.Errorf("could not create request, %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+v.config.ApiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post request, %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errText, _ := io.ReadAll(resp.Body)
		length := min(len(errText), 100)
		return nil, fmt.Errorf("unexpected status code, %d, err: %s", resp.StatusCode, string(errText[:length]))
	}

	var respModel response
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode response, %w", err)
	}
	if len(respModel.Data) == 0 {
		return nil, fmt.Errorf("no data in response")
	}

	return respModel.Data[0].Embedding, nil
}
