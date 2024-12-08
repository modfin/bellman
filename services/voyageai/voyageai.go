package voyageai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
)

var requestNo int64

type VoyageAI struct {
	apiKey string
	Log    *slog.Logger `json:"-"`
}

func (g *VoyageAI) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/voyage_ai] "+msg, args...)
}

func New(apiKey string) *VoyageAI {
	return &VoyageAI{apiKey: apiKey}
}

type localRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type localResponse struct {
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

func (v *VoyageAI) Embed(request embed.Request) (*embed.Response, error) {

	var reqc = atomic.AddInt64(&requestNo, 1)

	u := `https://api.voyageai.com/v1/embeddings`

	reqModel := localRequest{
		Input: []string{
			request.Text,
		},
		Model: request.Model.Name,
	}
	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal localRequest, %w", err)
	}

	req, err := http.NewRequest("POST", u, bytes.NewReader(jsonReq))
	if err != nil {
		return nil, fmt.Errorf("could not create localRequest, %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+v.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post localRequest, %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errText, _ := io.ReadAll(resp.Body)
		length := min(len(errText), 100)
		return nil, fmt.Errorf("unexpected status code, %d, err: %s", resp.StatusCode, string(errText[:length]))
	}

	var respModel localResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode localResponse, %w", err)
	}
	if len(respModel.Data) == 0 {
		return nil, fmt.Errorf("no data in localResponse")
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", respModel.Usage.TotalTokens)

	return &embed.Response{
		Embedding: respModel.Data[0].Embedding,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: respModel.Usage.TotalTokens,
		},
	}, nil
}

func (g *VoyageAI) SetLogger(logger *slog.Logger) *VoyageAI {
	g.Log = logger
	return g

}
