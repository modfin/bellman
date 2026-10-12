package openai_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modfin/bellman/models/decide"
	"github.com/modfin/bellman/services/openai"
)

func decideAgainst(t *testing.T, body string) (*decide.Response, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	req := decide.NewRequest(context.Background(), openai.DecideModel_gpt6_luna, decide.Text("I was charged twice")).Ask(
		decide.Predicate("refund", "Is the customer asking for a refund?"),
		decide.Choice("department", "Which department should handle this?",
			decide.Option{Value: "billing"}, decide.Option{Value: "other"}),
	)
	return openai.New("").SetBaseURL(srv.URL).Decide(req)
}

func TestDecideRefusalTypeComesFromName(t *testing.T) {
	// Answers out of question order, refusal first.
	res, err := decideAgainst(t, `{"answers":[
		{"type":"refusal","name":"department"},
		{"type":"predicate","name":"refund","probability":0.9}
	]}`)
	if err != nil {
		t.Fatal(err)
	}
	if a := res.Answers["department"]; !a.Refused || a.Type != decide.TypeChoice {
		t.Fatalf("expected refused choice, got %+v", a)
	}
	if _, err := res.Choice("department"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("expected refused error, got %v", err)
	}
}

func TestDecideRejectsMismatchedAnswerType(t *testing.T) {
	_, err := decideAgainst(t, `{"answers":[
		{"type":"predicate","name":"refund","probability":0.9},
		{"type":"score","name":"department","score":1}
	]}`)
	if err == nil || !strings.Contains(err.Error(), `has type "score"`) {
		t.Fatalf("expected type mismatch error, got %v", err)
	}
}
