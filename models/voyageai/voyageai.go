package voyageai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"io"
	"net/http"
)

type VoyageAI struct {
	apiKey string
}

func New(apiKey string) *VoyageAI {
	return &VoyageAI{apiKey: apiKey}
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

func (v *VoyageAI) Embed(text string, model bellman.EmbedModel) ([]float64, error) {

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

	req.Header.Set("Authorization", "Bearer "+v.apiKey)
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
