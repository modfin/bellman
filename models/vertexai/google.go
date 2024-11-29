package vertexai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman"
	"golang.org/x/oauth2"
	"net/http"

	"golang.org/x/oauth2/google"
)

type GoogleEmbedRequest struct {
	Instances []struct {
		TaskType string `json:"task_type"`
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

func (g *Google) Embed(text string, model string) ([]float64, error) {
	req := GoogleEmbedRequest{
		Instances: []struct {
			TaskType string `json:"task_type"`
			Content  string `json:"content"`
		}{
			{
				TaskType: model,
				Content:  text,
			},
		},
	}

	u := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		g.config.Region, g.config.Project, g.config.Region, model)

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
	err = json.NewDecoder(resp.Body).Decode(&embeddings)

	if err != nil {
		return nil, fmt.Errorf("could not decode google response, %w", err)
	}
	if len(embeddings.Predictions) != 1 {
		return nil, fmt.Errorf("wrong number of predictions, %d, expected 1", len(embeddings.Predictions))
	}

	return embeddings.Predictions[0].Embeddings.Values, nil
}

func (g *Google) Generate(options ...bellman.GeneratorOption) bellman.Generator {
	var gen bellman.Generator = &generator{
		g:           g,
		topP:        -1,
		temperature: -1,
		maxTokens:   -1,
	}
	for _, o := range options {
		gen = o(gen)
	}
	return gen

}
