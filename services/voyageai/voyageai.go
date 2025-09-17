package voyageai

import (
	"bytes"
	"context"
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

func (v *VoyageAI) log(msg string, args ...any) {
	if v.Log == nil {
		return
	}
	v.Log.Debug("[bellman/voyage_ai] "+msg, args...)
}

func New(apiKey string) *VoyageAI {
	return &VoyageAI{apiKey: apiKey}
}

type localRequest struct {
	Input     []string `json:"input"`
	Model     string   `json:"model"`
	InputType string   `json:"input_type,omitempty"`
}
type responseData struct {
	Object    string         `json:"object"`
	Embedding []float64      `json:"embedding"`
	Index     int            `json:"index"`
	Data      []responseData `json:"data"`
}
type localResponse struct {
	Object string         `json:"object"`
	Data   []responseData `json:"data"`
	Model  string         `json:"model"`
	Usage  struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
}
type localContextualizedRequest struct {
	Inputs    [][]string `json:"inputs"`
	Model     string     `json:"model"`
	InputType string     `json:"input_type,omitempty"`
}

func (v *VoyageAI) Provider() string {
	return Provider
}

func (v *VoyageAI) Embed(request *embed.Request) (*embed.Response, error) {

	var reqc = atomic.AddInt64(&requestNo, 1)

	u := `https://api.voyageai.com/v1/embeddings`

	if len(request.Texts) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	if len(request.Texts) > 1000 {
		// https://docs.voyageai.com/reference/embeddings-api
		return nil, fmt.Errorf("too many texts provided, max 1000")
	}
	reqModel := localRequest{
		Input: request.Texts,
		Model: request.Model.Name,
	}
	switch request.Model.Type {
	case embed.TypeQuery:
		reqModel.InputType = "query"
	case embed.TypeDocument:
		reqModel.InputType = "document"
	}

	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal localRequest, %w", err)
	}

	ctx := request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(jsonReq))
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
	if len(respModel.Data) != len(request.Texts) {
		return nil, fmt.Errorf("wrong number of embeddings, %d, expected %d", len(respModel.Data), len(request.Texts))
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", respModel.Usage.TotalTokens)

	embedResp := &embed.Response{
		Embeddings: make([][]float64, len(respModel.Data)),
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: respModel.Usage.TotalTokens,
		},
	}
	for idx, data := range respModel.Data {
		embedResp.Embeddings[idx] = data.Embedding
	}

	return embedResp, nil
}
func (v *VoyageAI) EmbedDocument(request *embed.DocumentRequest) (*embed.DocumentResponse, error) {

	var reqc = atomic.AddInt64(&requestNo, 1)

	u := `https://api.voyageai.com/v1/contextualizedembeddings`

	if len(request.DocumentChunks) == 0 {
		return nil, fmt.Errorf("no texts provided")
	}
	if len(request.DocumentChunks) > 1000 {
		// https://docs.voyageai.com/reference/embeddings-api
		return nil, fmt.Errorf("too many texts provided, max 1000")
	}
	reqModel := localContextualizedRequest{
		Inputs: [][]string{request.DocumentChunks},
		Model:  request.Model.Name,
	}
	switch request.Model.Type {
	case embed.TypeQuery:
		reqModel.InputType = "query"
	case embed.TypeDocument:
		reqModel.InputType = "document"
	}

	jsonReq, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal localRequest, %w", err)
	}

	ctx := request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(jsonReq))
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
		return nil, fmt.Errorf("no data in context localResponse")
	}
	if len(respModel.Data) != 1 {
		return nil, fmt.Errorf("no data in context localResponse")
	}
	if len(respModel.Data[0].Data) != len(request.DocumentChunks) {
		return nil, fmt.Errorf("wrong number of embeddings, %d, expected %d", len(respModel.Data[0].Data), len(request.DocumentChunks))
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", respModel.Usage.TotalTokens)

	embedResp := &embed.DocumentResponse{
		Embeddings: make([][]float64, len(respModel.Data[0].Data)),
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: respModel.Usage.TotalTokens,
		},
	}
	for idx, data := range respModel.Data[0].Data {
		embedResp.Embeddings[idx] = data.Embedding
	}

	return embedResp, nil
}

func (v *VoyageAI) SetLogger(logger *slog.Logger) *VoyageAI {
	v.Log = logger
	return v

}
