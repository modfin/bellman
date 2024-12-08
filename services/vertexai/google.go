package vertexai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"golang.org/x/oauth2"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"

	"golang.org/x/oauth2/google"
)

type GoogleEmbedRequest struct {
	Instances []struct {
		//TaskType string `json:"task_type"`
		//Title    string `json:"title"`
		Content string `json:"content"`
	} `json:"instances"`
}

type GoogleEmbedResponse struct {
	Predictions []struct {
		Embeddings struct {
			Statistics struct {
				Truncated  bool `json:"truncated"`
				TokenCount int  `json:"token_count"`
			} `json:"statistics"`
			Values []float64 `json:"values"`
		} `json:"embeddings"`
	} `json:"predictions"`
}

type GoogleConfig struct {
	Project    string `cli:"ai-google-project"`
	Region     string `cli:"ai-google-region"`
	Credential string `cli:"ai-google-credential"`
}

type Google struct {
	config GoogleConfig
	client *http.Client

	Log *slog.Logger `json:"-"`
}

func (g *Google) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/vertex_ai] "+msg, args...)
}

func New(config GoogleConfig) (*Google, error) {

	cred, err := google.CredentialsFromJSON(context.Background(), []byte(config.Credential), "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, fmt.Errorf("could not create google credentials, %w", err)
	}
	client := oauth2.NewClient(context.Background(), cred.TokenSource)

	return &Google{
		config: config,
		client: client,
	}, nil
}

func (g *Google) Embed(request embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)

	req := GoogleEmbedRequest{
		Instances: []struct {
			//TaskType string `json:"task_type"`
			Content string `json:"content"`
		}{
			{
				//TaskType: model.Name,
				Content: request.Text,
			},
		},
	}

	u := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		g.config.Region, g.config.Project, g.config.Region, request.Model.Name)

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("could not marshal google request, %w", err)
	}
	resp, err := g.client.Post(u, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not post google request, %w", err)
	}
	defer resp.Body.Close()

	var embeddings GoogleEmbedResponse
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read google response, %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code, %d, %s", resp.StatusCode, string(body))
	}

	err = json.Unmarshal(body, &embeddings)

	if err != nil {
		return nil, fmt.Errorf("could not decode google response, %w", err)
	}
	if len(embeddings.Predictions) != 1 {
		return nil, fmt.Errorf("wrong number of predictions, %d, expected 1", len(embeddings.Predictions))
	}

	g.log("[embed] response", "request", reqc, "token-total", embeddings.Predictions[0].Embeddings.Statistics.TokenCount)

	return &embed.Response{
		Embedding: embeddings.Predictions[0].Embeddings.Values,
		Metadata: models.Metadata{
			Model:       request.Model.FQN(),
			TotalTokens: embeddings.Predictions[0].Embeddings.Statistics.TokenCount,
		},
	}, nil
}

func (g *Google) Generator(options ...gen.Option) *gen.Generator {

	var gen = &gen.Generator{
		Prompter: &generator{
			google: g,
		},
		Request: gen.Request{
			Temperature: -1,
			MaxTokens:   -1,
			TopP:        -1,
		},
	}

	for _, o := range options {
		gen = o(gen)
	}
	return gen

}

func (g *Google) SetLogger(logger *slog.Logger) *Google {
	g.Log = logger
	return g

}