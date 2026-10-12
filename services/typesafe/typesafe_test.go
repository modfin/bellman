package typesafe_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modfin/bellman/models/decide"
	"github.com/modfin/bellman/services/typesafe"
)

func decideAgainst(t *testing.T, body string) (*decide.Response, error) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	req := decide.NewRequest(context.Background(), typesafe.DecideModel_jev_latest, decide.Text("I was charged twice")).Ask(
		decide.Predicate("refund", "Is the customer asking for a refund?"),
		decide.Score("urgency", "How urgent is this?",
			decide.Level{Label: "low"}, decide.Level{Label: "high"}),
	)
	return typesafe.New("").SetBaseURL(srv.URL).Decide(req)
}

func TestDecideRefusalKeepsOtherAnswers(t *testing.T) {
	res, err := decideAgainst(t, `{"answers":{
		"refund":  {"type":"noul","noul":0.9},
		"urgency": {"type":"refusal"}
	}}`)
	if err != nil {
		t.Fatal(err)
	}
	if p, err := res.Probability("refund"); err != nil || p != 0.9 {
		t.Fatalf("expected refund 0.9, got %v, %v", p, err)
	}
	if _, err := res.Score("urgency"); err == nil || !strings.Contains(err.Error(), "refused") {
		t.Fatalf("expected refused error, got %v", err)
	}
}

func TestDecideRejectsMismatchedAnswerType(t *testing.T) {
	_, err := decideAgainst(t, `{"answers":{
		"refund":  {"type":"noul","noul":0.9},
		"urgency": {"type":"choice","choice":"high","probabilities":{"high":1}}
	}}`)
	if err == nil || !strings.Contains(err.Error(), `has type "choice"`) {
		t.Fatalf("expected type mismatch error, got %v", err)
	}
}
