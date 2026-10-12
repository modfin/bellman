package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/decide"
)

// https://developers.openai.com/api/docs/guides/decisions
//
// POST /v1/decisions with a shared input and an array of typed questions.
// Predicate questions carry no criteria fields, so bellman folds any
// True/False descriptions into the instructions.

type decisionChoice struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

type decisionLevel struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type decisionQuestion struct {
	Type         string           `json:"type"` // "predicate" | "choice" | "score"
	Name         string           `json:"name"`
	Instructions string           `json:"instructions"`
	Choices      []decisionChoice `json:"choices,omitempty"`
	Levels       []decisionLevel  `json:"levels,omitempty"`
}

type decisionRequest struct {
	Model     string             `json:"model"`
	Input     []messageItem      `json:"input"`
	Questions []decisionQuestion `json:"questions"`
}

type decisionProbability struct {
	Value       json.RawMessage `json:"value"` // string for choice, integer index for score
	Label       string          `json:"label,omitempty"`
	Probability float64         `json:"probability"`
}

type decisionAnswer struct {
	Type          string                `json:"type"` // "predicate" | "choice" | "score" | "refusal"
	Name          string                `json:"name"`
	Probability   *float64              `json:"probability,omitempty"`
	Choice        json.RawMessage       `json:"choice,omitempty"`
	Score         *float64              `json:"score,omitempty"`
	Probabilities []decisionProbability `json:"probabilities,omitempty"`
	Confidence    *float64              `json:"confidence,omitempty"`
}

type decisionUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

type decisionResponse struct {
	Model   string           `json:"model"`
	Answers []decisionAnswer `json:"answers"`
	Usage   *decisionUsage   `json:"usage,omitempty"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (g *OpenAI) Decide(request *decide.Request) (*decide.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)

	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid decide request, %w", err)
	}

	reqModel := decisionRequest{
		Model: request.Model.Name,
		Input: []messageItem{{Role: "user", Content: decisionContent(request.State)}},
	}
	for _, q := range request.Questions {
		dq := decisionQuestion{
			Type:         string(q.Type),
			Name:         q.Name,
			Instructions: q.Instructions,
		}
		switch q.Type {
		case decide.TypePredicate:
			dq.Instructions = predicateInstructions(q)
		case decide.TypeChoice:
			for _, o := range q.Options {
				dq.Choices = append(dq.Choices, decisionChoice{Value: o.Value, Description: o.Description})
			}
		case decide.TypeScore:
			for _, l := range q.Levels {
				dq.Levels = append(dq.Levels, decisionLevel{Label: l.Label, Description: l.Description})
			}
		}
		reqModel.Questions = append(reqModel.Questions, dq)
	}

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s request, %w", g.provider, err)
	}

	u, err := url.JoinPath(g.getBaseURL(request.Model.Name), "/v1/decisions")
	if err != nil {
		return nil, fmt.Errorf("could not construct decisions URL, %w", err)
	}

	req, err := http.NewRequestWithContext(request.Context(), "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create %s request, %w", g.provider, err)
	}
	if g.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+g.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	g.log("[decide] request",
		"request", reqc,
		"model", request.Model.FQN(),
		"questions", len(request.Questions),
		"images", request.HasImages(),
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post %s request, %w", g.provider, err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read %s response, %w", g.provider, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code, %d: err %s", resp.StatusCode, string(body))
	}

	var respModel decisionResponse
	if err := json.Unmarshal(body, &respModel); err != nil {
		return nil, fmt.Errorf("could not decode %s response, %w", g.provider, err)
	}
	if respModel.Error != nil && respModel.Error.Message != "" {
		return nil, fmt.Errorf("%s decisions error: %s", g.provider, respModel.Error.Message)
	}

	// Answers come back in question order; name is echoed but fall back to
	// position in case it is omitted.
	response := &decide.Response{
		Answers: make(map[string]decide.Answer, len(respModel.Answers)),
		Metadata: models.Metadata{
			Model: request.Model.FQN(),
		},
	}
	if respModel.Usage != nil {
		response.Metadata.InputTokens = respModel.Usage.InputTokens
		response.Metadata.OutputTokens = respModel.Usage.OutputTokens
		response.Metadata.TotalTokens = respModel.Usage.TotalTokens
		if response.Metadata.TotalTokens == 0 {
			response.Metadata.TotalTokens = respModel.Usage.InputTokens + respModel.Usage.OutputTokens
		}
	}
	if respModel.Model != "" && respModel.Model != request.Model.Name {
		response.Metadata.Other = map[string]any{"resolved_model": respModel.Model}
	}

	for i, da := range respModel.Answers {
		name := da.Name
		if name == "" && i < len(request.Questions) {
			name = request.Questions[i].Name
		}
		a := decide.Answer{Name: name}
		switch da.Type {
		case "refusal":
			a.Refused = true
			if i < len(request.Questions) {
				a.Type = request.Questions[i].Type
			}
		case "predicate":
			a.Type = decide.TypePredicate
			if da.Probability != nil {
				a.Probability = *da.Probability
			}
		case "choice":
			a.Type = decide.TypeChoice
			a.Choice = rawString(da.Choice)
			a.Probabilities = make(map[string]float64, len(da.Probabilities))
			for _, p := range da.Probabilities {
				a.Probabilities[rawString(p.Value)] = p.Probability
			}
			if da.Confidence != nil {
				a.Confidence = *da.Confidence
			}
		case "score":
			a.Type = decide.TypeScore
			if da.Score != nil {
				a.Score = *da.Score
			}
			a.Probabilities = make(map[string]float64, len(da.Probabilities))
			for j, p := range da.Probabilities {
				key := rawString(p.Value)
				if key == "" {
					key = strconv.Itoa(j)
				}
				a.Probabilities[key] = p.Probability
			}
			if da.Confidence != nil {
				a.Confidence = *da.Confidence
			}
		default:
			return nil, fmt.Errorf("unknown answer type %q for question %q", da.Type, name)
		}
		response.Answers[name] = a
	}

	for _, q := range request.Questions {
		if _, ok := response.Answers[q.Name]; !ok {
			return nil, fmt.Errorf("no answer for question %q in %s response", q.Name, g.provider)
		}
	}

	g.log("[decide] response",
		"request", reqc,
		"model", request.Model.FQN(),
		"answers", len(response.Answers),
		"token-input", response.Metadata.InputTokens,
		"token-total", response.Metadata.TotalTokens,
	)

	return response, nil
}

func decisionContent(state []decide.Part) []messageContent {
	var content []messageContent
	for _, p := range state {
		switch p.Type {
		case decide.PartImage:
			content = append(content, messageContent{Type: "input_image", ImageURL: new(p.DataURL())})
		default:
			content = append(content, messageContent{Type: "input_text", Text: new(p.AsText())})
		}
	}
	return content
}

func predicateInstructions(q decide.Question) string {
	if q.True == "" && q.False == "" {
		return q.Instructions
	}
	var b strings.Builder
	b.WriteString(q.Instructions)
	if q.True != "" {
		b.WriteString("\nTrue when: ")
		b.WriteString(q.True)
	}
	if q.False != "" {
		b.WriteString("\nFalse when: ")
		b.WriteString(q.False)
	}
	return b.String()
}

// rawString renders a JSON scalar as the string bellman keys answers by: a
// JSON string loses its quotes, anything else (number, bool) keeps its
// literal form.
func rawString(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
