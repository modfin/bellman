// Package decide is the third model type in bellman, next to gen and embed.
//
// A decision model does not generate text. It is handed a shared State and a
// set of typed Questions and returns one Answer per question, each carrying a
// probability distribution instead of prose. This is the shape of OpenAI's
// Decisions API and TypeSafe's Jev (the "System One" API), and it suits
// classification, routing, scoring and guardrails where a generative model is
// slower, dearer and harder to threshold.
//
// Three question types exist:
//
//   - Predicate: a yes/no condition. The answer is P(true).
//   - Choice: pick one of up to 255 unordered options. The answer is the chosen
//     option, a probability per option and a confidence.
//   - Score: place the state on 2 to 10 ordered levels. The answer is the
//     probability-weighted level index, a probability per level and a confidence.
//
// Confidence, where returned, describes how peaked the distribution is. It is
// not a calibrated measure of correctness; thresholds are the caller's job.
package decide

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/modfin/bellman/models"
)

// Decider is implemented by every provider that can answer decision requests,
// and by the bellman client so a bellmand instance can be used in its place.
type Decider interface {
	Provider() string
	Decide(req *Request) (*Response, error)
}

type Model struct {
	Provider string `json:"provider"`
	Name     string `json:"name"`

	Description string `json:"description,omitempty"`

	InputMaxTokens int `json:"input_max_tokens,omitempty"`

	// SupportImages reports whether image parts may be included in the state.
	SupportImages bool `json:"support_images,omitempty"`
	// SupportJSONState reports whether a single JSON part is forwarded to the
	// provider as a structured object rather than flattened into text.
	SupportJSONState bool `json:"support_json_state,omitempty"`

	Config map[string]any `json:"config,omitempty"`
}

func (m Model) FQN() string {
	return m.String()
}

func (m Model) String() string {
	return m.Provider + "/" + m.Name
}

func ToModel(fqn string) (Model, error) {
	provider, name, found := strings.Cut(fqn, "/")
	if !found {
		return Model{}, errors.New("invalid fqn, did not find a '/' separating provider and model")
	}
	return Model{
		Provider: provider,
		Name:     name,
	}, nil
}

// QuestionType names the three decision primitives.
type QuestionType string

const (
	TypePredicate QuestionType = "predicate"
	TypeChoice    QuestionType = "choice"
	TypeScore     QuestionType = "score"
)

const (
	MinChoiceOptions = 2
	MaxChoiceOptions = 255
	MinScoreLevels   = 2
	MaxScoreLevels   = 10
)

// Option is one unordered alternative of a Choice question.
type Option struct {
	Value       string `json:"value"`
	Description string `json:"description,omitempty"`
}

func NewOption(value, description string) Option {
	return Option{Value: value, Description: description}
}

// Level is one rung of a Score question. Levels are ordered lowest to highest
// and are addressed by their zero-based index in the answer.
type Level struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

func NewLevel(label, description string) Level {
	return Level{Label: label, Description: description}
}

// Text returns the level as a single string for providers that take one
// description per level.
func (l Level) Text() string {
	switch {
	case l.Description == "":
		return l.Label
	case l.Label == "":
		return l.Description
	default:
		return l.Label + ": " + l.Description
	}
}

type Question struct {
	Name         string       `json:"name"`
	Type         QuestionType `json:"type"`
	Instructions string       `json:"instructions"`

	// True and False optionally spell out what a yes and a no mean for a
	// Predicate question.
	True  string `json:"true,omitempty"`
	False string `json:"false,omitempty"`

	Options []Option `json:"options,omitempty"` // Choice
	Levels  []Level  `json:"levels,omitempty"`  // Score, lowest to highest
}

// Predicate asks a yes/no question. The answer carries P(true) in
// Answer.Probability.
func Predicate(name, instructions string) Question {
	return Question{Name: name, Type: TypePredicate, Instructions: instructions}
}

// WithCriteria describes what a yes and a no mean for a Predicate question.
func (q Question) WithCriteria(whenTrue, whenFalse string) Question {
	q.True = whenTrue
	q.False = whenFalse
	return q
}

// Choice asks the model to pick one of the given options. Include a fallback
// such as "other" when the list may not cover every input.
func Choice(name, instructions string, options ...Option) Question {
	return Question{Name: name, Type: TypeChoice, Instructions: instructions, Options: options}
}

// Score asks the model to place the state on an ordered scale, given lowest
// to highest.
func Score(name, instructions string, levels ...Level) Question {
	return Question{Name: name, Type: TypeScore, Instructions: instructions, Levels: levels}
}

func (q Question) validate() error {
	if q.Name == "" {
		return errors.New("question name is required")
	}
	if q.Instructions == "" {
		return fmt.Errorf("question %q: instructions are required", q.Name)
	}
	switch q.Type {
	case TypePredicate:
		if len(q.Options) > 0 || len(q.Levels) > 0 {
			return fmt.Errorf("question %q: predicate takes neither options nor levels", q.Name)
		}
	case TypeChoice:
		if len(q.Options) < MinChoiceOptions || len(q.Options) > MaxChoiceOptions {
			return fmt.Errorf("question %q: choice needs %d to %d options, got %d", q.Name, MinChoiceOptions, MaxChoiceOptions, len(q.Options))
		}
		seen := map[string]bool{}
		for _, o := range q.Options {
			if o.Value == "" {
				return fmt.Errorf("question %q: option value is required", q.Name)
			}
			if seen[o.Value] {
				return fmt.Errorf("question %q: duplicate option %q", q.Name, o.Value)
			}
			seen[o.Value] = true
		}
	case TypeScore:
		if len(q.Levels) < MinScoreLevels || len(q.Levels) > MaxScoreLevels {
			return fmt.Errorf("question %q: score needs %d to %d levels, got %d", q.Name, MinScoreLevels, MaxScoreLevels, len(q.Levels))
		}
		for i, l := range q.Levels {
			if l.Text() == "" {
				return fmt.Errorf("question %q: level %d has neither label nor description", q.Name, i)
			}
		}
	default:
		return fmt.Errorf("question %q: unknown type %q", q.Name, q.Type)
	}
	return nil
}

// PartType names what a state Part holds.
type PartType string

const (
	PartText  PartType = "text"
	PartJSON  PartType = "json"
	PartImage PartType = "image"
)

// Part is one piece of the shared state every question is asked against.
type Part struct {
	Type PartType `json:"type"`

	Text string          `json:"text,omitempty"`
	JSON json.RawMessage `json:"json,omitempty"`

	// Mime and Data (base64) carry an image. Hosted URLs are not accepted by
	// the providers, so only inline data is supported.
	Mime string `json:"mime,omitempty"`
	Data string `json:"data,omitempty"`

	err error
}

// Text adds free text to the state.
func Text(text string) Part {
	return Part{Type: PartText, Text: text}
}

// JSON adds a structured object to the state. Providers that accept JSON
// state receive it as an object so questions can refer to its fields;
// others receive it serialized as text. A value that cannot be marshaled
// surfaces as an error from Request.Validate.
func JSON(v any) Part {
	if raw, ok := v.(json.RawMessage); ok {
		return Part{Type: PartJSON, JSON: raw}
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return Part{Type: PartJSON, err: fmt.Errorf("could not marshal json state: %w", err)}
	}
	return Part{Type: PartJSON, JSON: raw}
}

// Image adds an inline image to the state.
func Image(mime string, data []byte) Part {
	return Part{Type: PartImage, Mime: mime, Data: base64.StdEncoding.EncodeToString(data)}
}

// DataURL renders an image part as a data: URL.
func (p Part) DataURL() string {
	mime := p.Mime
	if mime == "" {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + p.Data
}

// AsText flattens a text or json part to a string. Image parts yield "".
func (p Part) AsText() string {
	switch p.Type {
	case PartText:
		return p.Text
	case PartJSON:
		return string(p.JSON)
	}
	return ""
}

type Request struct {
	Ctx context.Context `json:"-"`

	Model     Model      `json:"model"`
	State     []Part     `json:"state"`
	Questions []Question `json:"questions"`
}

// NewRequest starts a request against model with the given state parts. Add
// questions with Ask.
func NewRequest(ctx context.Context, model Model, state ...Part) *Request {
	return &Request{Ctx: ctx, Model: model, State: state}
}

// Ask appends questions to the request and returns it for chaining.
func (r *Request) Ask(questions ...Question) *Request {
	r.Questions = append(r.Questions, questions...)
	return r
}

// Context returns the request context, defaulting to context.Background.
func (r *Request) Context() context.Context {
	if r.Ctx == nil {
		return context.Background()
	}
	return r.Ctx
}

// HasImages reports whether any state part is an image.
func (r *Request) HasImages() bool {
	for _, p := range r.State {
		if p.Type == PartImage {
			return true
		}
	}
	return false
}

// StateText flattens the text and json parts into one string, separated by
// blank lines. Image parts are skipped.
func (r *Request) StateText() string {
	var parts []string
	for _, p := range r.State {
		if t := p.AsText(); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, "\n\n")
}

// Validate checks the request is well-formed before it is sent anywhere.
// Providers call it; callers may too.
func (r *Request) Validate() error {
	if r.Model.Name == "" {
		return errors.New("model name is required")
	}
	if len(r.State) == 0 {
		return errors.New("state is required")
	}
	for i, p := range r.State {
		if p.err != nil {
			return fmt.Errorf("state part %d: %w", i, p.err)
		}
		switch p.Type {
		case PartText, PartJSON:
			if p.AsText() == "" {
				return fmt.Errorf("state part %d: empty %s part", i, p.Type)
			}
		case PartImage:
			if p.Data == "" {
				return fmt.Errorf("state part %d: image part has no data", i)
			}
		default:
			return fmt.Errorf("state part %d: unknown part type %q", i, p.Type)
		}
	}
	if len(r.Questions) == 0 {
		return errors.New("at least one question is required")
	}
	seen := map[string]bool{}
	for _, q := range r.Questions {
		if err := q.validate(); err != nil {
			return err
		}
		if seen[q.Name] {
			return fmt.Errorf("duplicate question name %q", q.Name)
		}
		seen[q.Name] = true
	}
	return nil
}

// Answer is the model's reply to one question. Which fields are set depends
// on Type: Probability for a predicate; Choice, Probabilities and Confidence
// for a choice; Score, Probabilities and Confidence for a score. A refused
// question has Refused set and nothing else.
type Answer struct {
	Name string       `json:"name"`
	Type QuestionType `json:"type"`

	Refused bool `json:"refused,omitempty"`

	// Probability is P(true) for a predicate.
	Probability float64 `json:"probability,omitempty"`

	// Choice is the option value with the highest probability.
	Choice string `json:"choice,omitempty"`

	// Score is the probability-weighted average of the zero-based level
	// indices, so it may fall between two levels.
	Score float64 `json:"score,omitempty"`

	// Probabilities is keyed by option value for a choice and by zero-based
	// level index ("0", "1", ...) for a score. Values sum to roughly 1.
	Probabilities map[string]float64 `json:"probabilities,omitempty"`

	// Confidence in [0, 1] describes how concentrated Probabilities is, as
	// reported by the provider. It is not calibrated correctness.
	Confidence float64 `json:"confidence,omitempty"`
}

// Level returns the nearest level index for a score answer.
func (a Answer) Level() int {
	if a.Type != TypeScore {
		return 0
	}
	return int(a.Score + 0.5)
}

// ProbabilityOf returns the probability assigned to an option value (choice)
// or a level index (score).
func (a Answer) ProbabilityOf(key string) float64 {
	return a.Probabilities[key]
}

// LevelProbability returns the probability assigned to a level index of a
// score answer.
func (a Answer) LevelProbability(index int) float64 {
	return a.Probabilities[strconv.Itoa(index)]
}

type Response struct {
	Answers  map[string]Answer `json:"answers"`
	Metadata models.Metadata   `json:"metadata,omitempty"`
}

// Answer looks up the answer to a named question.
func (r *Response) Answer(name string) (Answer, bool) {
	a, ok := r.Answers[name]
	return a, ok
}

// Probability returns P(true) for a predicate question, or an error if the
// question is missing, refused or not a predicate.
func (r *Response) Probability(name string) (float64, error) {
	a, err := r.answerOfType(name, TypePredicate)
	if err != nil {
		return 0, err
	}
	return a.Probability, nil
}

// Choice returns the chosen option of a choice question.
func (r *Response) Choice(name string) (string, error) {
	a, err := r.answerOfType(name, TypeChoice)
	if err != nil {
		return "", err
	}
	return a.Choice, nil
}

// Score returns the weighted level index of a score question.
func (r *Response) Score(name string) (float64, error) {
	a, err := r.answerOfType(name, TypeScore)
	if err != nil {
		return 0, err
	}
	return a.Score, nil
}

func (r *Response) answerOfType(name string, t QuestionType) (Answer, error) {
	a, ok := r.Answers[name]
	if !ok {
		return Answer{}, fmt.Errorf("no answer for question %q", name)
	}
	if a.Refused {
		return Answer{}, fmt.Errorf("question %q was refused", name)
	}
	if a.Type != t {
		return Answer{}, fmt.Errorf("question %q is a %s, not a %s", name, a.Type, t)
	}
	return a, nil
}
