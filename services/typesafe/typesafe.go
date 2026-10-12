package typesafe

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/decide"
)

// https://docs.typesafe.ai/introduction/quickstart
//
// POST /v1/systemone
//
//	{
//	  "state": "<text>" | {<object>},
//	  "model": "jev-latest",
//	  "questions": {
//	    "<name>": {"type": "noul",   "instructions": "...", "criteria": {"true": "...", "false": "..."}},
//	    "<name>": {"type": "choice", "instructions": "...", "criteria": {"<value>": "<description>"}},
//	    "<name>": {"type": "score",  "instructions": "...", "criteria": ["<lowest>", ..., "<highest>"]}
//	  }
//	}
//
// The same wire format is served by Ollama 0.35+ and other System One
// runtimes, so SetBaseURL is enough to point this client at one of them.

const defaultBaseURL = "https://api.typesafe.ai"

var requestNo int64

type TypeSafe struct {
	apiKey  string
	baseURL string
	Log     *slog.Logger `json:"-"`
}

func New(apiKey string) *TypeSafe {
	return &TypeSafe{apiKey: apiKey}
}

func (t *TypeSafe) SetBaseURL(baseURL string) *TypeSafe {
	t.baseURL = baseURL
	return t
}

func (t *TypeSafe) SetLogger(logger *slog.Logger) *TypeSafe {
	t.Log = logger
	return t
}

func (t *TypeSafe) log(msg string, args ...any) {
	if t.Log == nil {
		return
	}
	t.Log.Debug("[bellman/typesafe] "+msg, args...)
}

func (t *TypeSafe) Provider() string {
	return Provider
}

func (t *TypeSafe) getBaseURL() string {
	if t.baseURL != "" {
		return t.baseURL
	}
	return defaultBaseURL
}

type systemOneQuestion struct {
	Type         string `json:"type"` // "noul" | "choice" | "score"
	Instructions string `json:"instructions"`
	Criteria     any    `json:"criteria,omitempty"`
}

type systemOneRequest struct {
	State     any                          `json:"state"`
	Model     string                       `json:"model"`
	Questions map[string]systemOneQuestion `json:"questions"`
}

type systemOneAnswer struct {
	Type          string             `json:"type"`
	Noul          *float64           `json:"noul,omitempty"`
	Choice        string             `json:"choice,omitempty"`
	Score         *float64           `json:"score,omitempty"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Confidence    *float64           `json:"confidence,omitempty"`
}

type systemOneResponse struct {
	Model   string                     `json:"model"`
	Answers map[string]systemOneAnswer `json:"answers"`
	Usage   struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (t *TypeSafe) Decide(request *decide.Request) (*decide.Response, error) {
	var reqc = atomic.AddInt64(&requestNo, 1)

	if err := request.Validate(); err != nil {
		return nil, fmt.Errorf("invalid decide request, %w", err)
	}
	if request.HasImages() {
		return nil, fmt.Errorf("%s does not accept image state", Provider)
	}

	reqModel := systemOneRequest{
		State:     stateOf(request),
		Model:     request.Model.Name,
		Questions: make(map[string]systemOneQuestion, len(request.Questions)),
	}
	for _, q := range request.Questions {
		sq := systemOneQuestion{Instructions: q.Instructions}
		switch q.Type {
		case decide.TypePredicate:
			sq.Type = "noul"
			if q.True != "" || q.False != "" {
				criteria := map[string]string{}
				if q.True != "" {
					criteria["true"] = q.True
				}
				if q.False != "" {
					criteria["false"] = q.False
				}
				sq.Criteria = criteria
			}
		case decide.TypeChoice:
			sq.Type = "choice"
			criteria := make(map[string]*string, len(q.Options))
			for _, o := range q.Options {
				if o.Description == "" {
					criteria[o.Value] = nil
					continue
				}
				d := o.Description
				criteria[o.Value] = &d
			}
			sq.Criteria = criteria
		case decide.TypeScore:
			sq.Type = "score"
			criteria := make([]string, 0, len(q.Levels))
			for _, l := range q.Levels {
				criteria = append(criteria, l.Text())
			}
			sq.Criteria = criteria
		}
		reqModel.Questions[q.Name] = sq
	}

	body, err := json.Marshal(reqModel)
	if err != nil {
		return nil, fmt.Errorf("could not marshal %s request, %w", Provider, err)
	}

	u, err := url.JoinPath(t.getBaseURL(), "/v1/systemone")
	if err != nil {
		return nil, fmt.Errorf("could not construct systemone URL, %w", err)
	}

	req, err := http.NewRequestWithContext(request.Context(), "POST", u, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("could not create %s request, %w", Provider, err)
	}
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}
	req.Header.Set("Content-Type", "application/json")

	t.log("[decide] request",
		"request", reqc,
		"model", request.Model.FQN(),
		"questions", len(request.Questions),
	)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not post %s request, %w", Provider, err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read %s response, %w", Provider, err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code, %d: err %s", resp.StatusCode, string(body))
	}

	var respModel systemOneResponse
	if err := json.Unmarshal(body, &respModel); err != nil {
		return nil, fmt.Errorf("could not decode %s response, %w", Provider, err)
	}

	response := &decide.Response{
		Answers: make(map[string]decide.Answer, len(respModel.Answers)),
		Metadata: models.Metadata{
			Model:        request.Model.FQN(),
			InputTokens:  respModel.Usage.InputTokens,
			OutputTokens: respModel.Usage.OutputTokens,
			TotalTokens:  respModel.Usage.InputTokens + respModel.Usage.OutputTokens,
		},
	}
	if respModel.Model != "" && respModel.Model != request.Model.Name {
		response.Metadata.Other = map[string]any{"resolved_model": respModel.Model}
	}

	for _, q := range request.Questions {
		sa, ok := respModel.Answers[q.Name]
		if !ok {
			return nil, fmt.Errorf("no answer for question %q in %s response", q.Name, Provider)
		}
		a := decide.Answer{Name: q.Name, Type: q.Type}
		switch sa.Type {
		case "noul":
			if sa.Noul != nil {
				a.Probability = *sa.Noul
			}
		case "choice":
			a.Choice = sa.Choice
			a.Probabilities = sa.Probabilities
			if sa.Confidence != nil {
				a.Confidence = *sa.Confidence
			}
		case "score":
			if sa.Score != nil {
				a.Score = *sa.Score
			}
			a.Probabilities = sa.Probabilities
			if sa.Confidence != nil {
				a.Confidence = *sa.Confidence
			}
		default:
			return nil, fmt.Errorf("unknown answer type %q for question %q", sa.Type, q.Name)
		}
		response.Answers[q.Name] = a
	}

	t.log("[decide] response",
		"request", reqc,
		"model", request.Model.FQN(),
		"resolved_model", respModel.Model,
		"answers", len(response.Answers),
		"token-input", response.Metadata.InputTokens,
		"token-total", response.Metadata.TotalTokens,
	)

	return response, nil
}

// stateOf forwards a lone JSON part as an object so questions can refer to
// its fields; anything else is flattened to text.
func stateOf(request *decide.Request) any {
	if len(request.State) == 1 && request.State[0].Type == decide.PartJSON {
		return request.State[0].JSON
	}
	return request.StateText()
}
