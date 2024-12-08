package bellman

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"
)

type Bellman struct {
	Log *slog.Logger `json:"-"`
	url string
	key Key
}

type Key struct {
	Name  string
	Token string
}

func (l Key) String() string {
	return l.Name + "_" + l.Token
}

func New(url string, key Key) *Bellman {
	return &Bellman{
		url: url,
		key: key,
	}

}

func (g *Bellman) log(msg string, args ...any) {
	if g.Log == nil {
		return
	}
	g.Log.Debug("[bellman/bellman] "+msg, args...)
}

var bellmanRequestNo int64

func (v *Bellman) EmbedModels() ([]embed.Model, error) {
	u, err := url.JoinPath(v.url, "embed", "models")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var models []embed.Model
	err = json.Unmarshal(body, &models)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}
	return models, nil
}

func (v *Bellman) GenModels() ([]gen.Model, error) {
	u, err := url.JoinPath(v.url, "gen", "models")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var models []gen.Model
	err = json.Unmarshal(body, &models)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}
	return models, nil
}

func (v *Bellman) Embed(request embed.Request) (*embed.Response, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	u, err := url.JoinPath(v.url, "embed")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", v.url, err)
	}

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}
	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.key.String())
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}

	var response embed.Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}

	v.log("[embed] response", "request", reqc, "model", request.Model.FQN(), "token-total", response.Metadata.TotalTokens)

	return &response, nil
}

func (a *Bellman) Generator(options ...gen.Option) *gen.Generator {
	var gen = &gen.Generator{
		Prompter: &generator{
			bellman: a,
		},
		Request: gen.Request{
			TopP:        -1,
			Temperature: 1,
			MaxTokens:   1024,
		},
	}
	for _, op := range options {
		gen = op(gen)
	}
	return gen
}

func (g *Bellman) SetLogger(logger *slog.Logger) *Bellman {
	g.Log = logger
	return g
}

type generator struct {
	bellman *Bellman
	request gen.Request
}

func (g *generator) SetRequest(request gen.Request) {
	g.request = request
}

func (g *generator) Prompt(conversation ...prompt.Prompt) (*gen.Response, error) {
	var reqc = atomic.AddInt64(&bellmanRequestNo, 1)

	u, err := url.JoinPath(g.bellman.url, "gen")
	if err != nil {
		return nil, fmt.Errorf("could not join url %s; %w", g.bellman.url, err)
	}
	request := gen.FullRequest{
		Request: g.request,
		Prompts: conversation,
	}

	toolBelt := map[string]*tools.Tool{}
	for _, tool := range g.request.Tools {
		toolBelt[tool.Name] = &tool
	}

	g.bellman.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolConfig != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
	)

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("could not marshal bellman request; %w", err)
	}
	req, err := http.NewRequest("POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create bellman request; %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.bellman.key.String())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post bellman request to %s; %w", u, err)
	}
	defer res.Body.Close()

	body, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read bellman response; %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code %d; %s", res.StatusCode, string(body))
	}
	response := gen.Response{}
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal bellman response; %w", err)
	}

	g.bellman.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", response.Metadata.InputTokens,
		"token-output", response.Metadata.OutputTokens,
		"token-total", response.Metadata.TotalTokens,
	)

	// adding reference to tools
	for i, _ := range response.Tools {
		tool := response.Tools[i]
		tool.Ref = toolBelt[tool.Name]
		response.Tools[i] = tool
	}

	return &response, nil

}
