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
	"regexp"
	"sync/atomic"

	"golang.org/x/oauth2/google"
)

type googleEmbedRequestInstance struct {
	TaskType string `json:"task_type,omitempty"`
	//Title    string `json:"title"`
	Content string `json:"content"`
}
type GoogleEmbedRequest struct {
	Instances []googleEmbedRequestInstance `json:"instances"`
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
	Project    string
	Region     string
	Credential string
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

	var client *http.Client
	var err error

	if config.Credential != "" {
		cred, err := google.CredentialsFromJSON(context.Background(), []byte(config.Credential), "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("could not create google credentials, %w", err)
		}
		client = oauth2.NewClient(context.Background(), cred.TokenSource)
	}
	if config.Credential == "" {
		client, err = google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("could not create google default client, %w", err)
		}
	}

	return &Google{
		config: config,
		client: client,
	}, nil
}

func (g *Google) Provider() string {
	return Provider
}

var projectIdPattern = regexp.MustCompile(`^[a-z]([a-z0-9-]{4,28}[a-z0-9])?$`)
var regionPattern = regexp.MustCompile(`^(global)|([a-z]+-[a-z]+[1-9][0-9]*)$`)
var modelNamePattern = regexp.MustCompile(`^[\w.-]+$`) // should probably be gemini-[\w.-]

func (g *Google) Embed(request embed.Request) (*embed.Response, error) {
	resp, err := g.EmbedMany(embed.RequestMany{
		Texts: []string{request.Text},
		Model: request.Model,
		Ctx:   request.Ctx,
	})
	if err != nil {
		return nil, err
	}
	if len(resp.Embeddings) != 1 {
		return nil, fmt.Errorf("wrong number of embeddings returned, %d, expected 1", len(resp.Embeddings))
	}
	return &embed.Response{
		Embedding: resp.Embeddings[0],
		Metadata:  resp.Metadata,
	}, nil
}

func (g *Google) EmbedMany(request embed.RequestMany) (*embed.ResponseMany, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)

	tasktype := ""
	switch request.Model.Type {
	case embed.TypeDocument:
		tasktype = string(TypeDocument)
	case embed.TypeQuery:
		tasktype = string(TypeQuery)
	default:
		tasktype = string(request.Model.Type)
	}
	if len(request.Texts) == 0 {
		return nil, fmt.Errorf("no texts provided to embed")
	}
	if len(request.Texts) > 250 {
		// https://cloud.google.com/vertex-ai/generative-ai/docs/embeddings/get-text-embeddings#api_limits
		return nil, fmt.Errorf("too many texts provided to embed, max is 250")
	}

	req := GoogleEmbedRequest{
		Instances: make([]googleEmbedRequestInstance, len(request.Texts)),
	}
	for idx, text := range request.Texts {
		req.Instances[idx] = googleEmbedRequestInstance{
			TaskType: tasktype,
			Content:  text,
		}
	}

	region := g.config.Region
	project := g.config.Project
	if len(request.Model.Config) > 0 {
		cfg := request.Model.Config
		r, ok := cfg["region"].(string)
		if ok {
			region = r
		}
		p, ok := cfg["project"].(string)
		if ok {
			project = p
		}
	}

	if !modelNamePattern.MatchString(request.Model.Name) {
		return nil, fmt.Errorf("model name %q contains invalid characters, only [\\w.-]+ is allowed", request.Model.Name)
	}

	if !regionPattern.MatchString(region) {
		return nil, fmt.Errorf("region %q contains invalid characters, only [a-z]+-[a-z]+[1-9][0-9]* or global is allowed", region)
	}

	if !projectIdPattern.MatchString(project) {
		return nil, fmt.Errorf("project %q contains invalid characters, only [a-z]([a-z0-9-]{4,28}[a-z0-9])? is allowed", project)
	}

	u := fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:predict",
		region, project, region, request.Model.Name)

	if region == "global" {
		u = fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/global/publishers/google/models/%s:predict",
			project, request.Model.Name)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("could not marshal google request, %w", err)
	}

	ctx := request.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	hreq, err := http.NewRequestWithContext(ctx, "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create google request, %w", err)
	}
	hreq.Header.Set("Content-Type", "application/json")
	resp, err := g.client.Do(hreq)
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
	if len(embeddings.Predictions) != len(request.Texts) {
		return nil, fmt.Errorf("wrong number of predictions, %d, expected %d	", len(embeddings.Predictions), len(request.Texts))
	}

	embedResp := &embed.ResponseMany{
		Embeddings: make([][]float64, len(embeddings.Predictions)),
		Metadata: models.Metadata{
			Model: request.Model.FQN(),
		},
	}
	for idx, prediction := range embeddings.Predictions {
		embedResp.Embeddings[idx] = prediction.Embeddings.Values
		embedResp.Metadata.TotalTokens += prediction.Embeddings.Statistics.TokenCount
	}

	g.log("[embed] response", "request", reqc, "token-total", embeddings.Predictions[0].Embeddings.Statistics.TokenCount)
	return embedResp, nil
}

func (g *Google) EmbedDocument(request embed.RequestDocument) (*embed.ResponseDocument, error) {
	return nil, fmt.Errorf("not supported by google embed models")
}

func (g *Google) Generator(options ...gen.Option) *gen.Generator {

	var gen = &gen.Generator{
		Prompter: &generator{
			google: g,
		},
		Request: gen.Request{},
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
