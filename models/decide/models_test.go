package decide

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func validRequest() *Request {
	return NewRequest(context.Background(), Model{Provider: "Mock", Name: "m"}, Text("I was charged twice")).Ask(
		Predicate("refund", "Is a refund requested?").WithCriteria("money back", "no money back"),
		Choice("department", "Which team?", NewOption("billing", "money"), NewOption("other", "")),
		Score("urgency", "How urgent?", NewLevel("low", ""), NewLevel("high", "today")),
	)
}

func TestValidateOK(t *testing.T) {
	if err := validRequest().Validate(); err != nil {
		t.Fatalf("expected valid request, got %v", err)
	}
}

func TestValidateErrors(t *testing.T) {
	cases := map[string]func(r *Request){
		"no model":          func(r *Request) { r.Model.Name = "" },
		"no state":          func(r *Request) { r.State = nil },
		"empty text":        func(r *Request) { r.State = []Part{Text("")} },
		"image no data":     func(r *Request) { r.State = []Part{{Type: PartImage}} },
		"bad part type":     func(r *Request) { r.State = []Part{{Type: "video", Text: "x"}} },
		"no questions":      func(r *Request) { r.Questions = nil },
		"duplicate name":    func(r *Request) { r.Questions[1].Name = "refund" },
		"missing name":      func(r *Request) { r.Questions[0].Name = "" },
		"no instructions":   func(r *Request) { r.Questions[0].Instructions = "" },
		"unknown type":      func(r *Request) { r.Questions[0].Type = "guess" },
		"predicate options": func(r *Request) { r.Questions[0].Options = []Option{NewOption("a", "")} },
		"one option":        func(r *Request) { r.Questions[1].Options = r.Questions[1].Options[:1] },
		"duplicate option":  func(r *Request) { r.Questions[1].Options[1].Value = "billing" },
		"empty option":      func(r *Request) { r.Questions[1].Options[1].Value = "" },
		"one level":         func(r *Request) { r.Questions[2].Levels = r.Questions[2].Levels[:1] },
		"blank level":       func(r *Request) { r.Questions[2].Levels[0] = Level{} },
		"too many levels": func(r *Request) {
			r.Questions[2].Levels = make([]Level, MaxScoreLevels+1)
			for i := range r.Questions[2].Levels {
				r.Questions[2].Levels[i] = NewLevel("l", "")
			}
		},
		"unmarshalable json": func(r *Request) { r.State = []Part{JSON(make(chan int))} },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validRequest()
			mutate(r)
			if err := r.Validate(); err == nil {
				t.Fatalf("expected validation error")
			}
		})
	}
}

func TestRequestJSONRoundTrip(t *testing.T) {
	r := validRequest()
	r.State = append(r.State, JSON(map[string]any{"order": "A-1"}), Image("image/png", []byte{1, 2, 3}))

	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), `"Ctx"`) {
		t.Fatalf("context leaked into json: %s", b)
	}

	var back Request
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatal(err)
	}
	if err := back.Validate(); err != nil {
		t.Fatalf("round-tripped request invalid: %v", err)
	}
	if len(back.State) != 3 || back.State[1].Type != PartJSON || back.State[2].Type != PartImage {
		t.Fatalf("state parts lost in round trip: %+v", back.State)
	}
	if back.State[2].DataURL() != "data:image/png;base64,AQID" {
		t.Fatalf("unexpected data url %q", back.State[2].DataURL())
	}
	if !back.HasImages() {
		t.Fatalf("expected HasImages")
	}
	if got := back.StateText(); got != "I was charged twice\n\n{\"order\":\"A-1\"}" {
		t.Fatalf("unexpected StateText %q", got)
	}
	if back.Questions[0].True != "money back" || back.Questions[2].Levels[1].Text() != "high: today" {
		t.Fatalf("question details lost: %+v", back.Questions)
	}
}

func TestResponseAccessors(t *testing.T) {
	res := &Response{Answers: map[string]Answer{
		"refund":     {Name: "refund", Type: TypePredicate, Probability: 0.9},
		"department": {Name: "department", Type: TypeChoice, Choice: "billing", Probabilities: map[string]float64{"billing": 0.8, "other": 0.2}, Confidence: 0.6},
		"urgency":    {Name: "urgency", Type: TypeScore, Score: 1.6, Probabilities: map[string]float64{"0": 0.1, "1": 0.2, "2": 0.7}},
		"blocked":    {Name: "blocked", Type: TypePredicate, Refused: true},
	}}

	if p, err := res.Probability("refund"); err != nil || p != 0.9 {
		t.Fatalf("Probability() = %v, %v", p, err)
	}
	if c, err := res.Choice("department"); err != nil || c != "billing" {
		t.Fatalf("Choice() = %v, %v", c, err)
	}
	if s, err := res.Score("urgency"); err != nil || s != 1.6 {
		t.Fatalf("Score() = %v, %v", s, err)
	}
	if a, _ := res.Answer("urgency"); a.Level() != 2 || a.LevelProbability(2) != 0.7 {
		t.Fatalf("Level()/LevelProbability() wrong: %+v", a)
	}
	if a, _ := res.Answer("department"); a.ProbabilityOf("other") != 0.2 {
		t.Fatalf("ProbabilityOf wrong")
	}
	if _, err := res.Probability("department"); err == nil {
		t.Fatalf("expected type mismatch error")
	}
	if _, err := res.Probability("blocked"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("expected refused error, got %v", err)
	}
	if _, err := res.Choice("missing"); err == nil {
		t.Fatalf("expected missing error")
	}
}

func TestToModel(t *testing.T) {
	m, err := ToModel("TypeSafe/jev-latest")
	if err != nil || m.Provider != "TypeSafe" || m.Name != "jev-latest" || m.FQN() != "TypeSafe/jev-latest" {
		t.Fatalf("ToModel() = %+v, %v", m, err)
	}
	if _, err := ToModel("nope"); err == nil {
		t.Fatalf("expected error for fqn without slash")
	}
}
