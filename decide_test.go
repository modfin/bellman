package bellman_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/decide"
)

var mockDecideModel = decide.Model{Provider: bellman.MockProvider, Name: "mock"}

func TestMockDecide(t *testing.T) {
	proxy := bellman.NewProxy()
	proxy.RegisterDecider(bellman.NewMock())

	res, err := proxy.Decide(decide.NewRequest(context.Background(), mockDecideModel,
		decide.Text("yes please, billing, this is high priority")).Ask(
		decide.Predicate("refund", "Refund?"),
		decide.Choice("department", "Team?", decide.NewOption("shipping", ""), decide.NewOption("billing", "")),
		decide.Score("urgency", "Urgency?", decide.NewLevel("low", ""), decide.NewLevel("medium", ""), decide.NewLevel("high", "")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if p, _ := res.Probability("refund"); p != 0.9 {
		t.Fatalf("predicate = %v", p)
	}
	if c, _ := res.Choice("department"); c != "billing" {
		t.Fatalf("choice = %v", c)
	}
	if s, _ := res.Score("urgency"); s != 2 {
		t.Fatalf("score = %v", s)
	}

	_, err = proxy.Decide(decide.NewRequest(context.Background(), decide.Model{Provider: "Nope", Name: "x"}, decide.Text("x")).Ask(decide.Predicate("a", "b")))
	if err == nil || !strings.Contains(err.Error(), "no client registerd") {
		t.Fatalf("expected unknown provider error, got %v", err)
	}
}

// TestClientDecide checks the bellman client marshals a decide request the way
// bellmand expects it and unmarshals the response it hands back.
func TestClientDecide(t *testing.T) {
	proxy := bellman.NewProxy()
	proxy.RegisterDecider(bellman.NewMock())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/decide" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test_token" {
			t.Errorf("unexpected auth header %q", got)
		}
		var req decide.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		res, err := proxy.Decide(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(res)
	}))
	defer srv.Close()

	client := bellman.New(srv.URL, bellman.Key{Name: "test", Token: "token"})
	var d decide.Decider = client

	res, err := d.Decide(decide.NewRequest(context.Background(), mockDecideModel, decide.JSON(map[string]string{"note": "yes"})).Ask(
		decide.Predicate("refund", "Refund?"),
	))
	if err != nil {
		t.Fatal(err)
	}
	if p, err := res.Probability("refund"); err != nil || p != 0.9 {
		t.Fatalf("Probability() = %v, %v", p, err)
	}
	if res.Metadata.Model != mockDecideModel.FQN() {
		t.Fatalf("metadata model = %q", res.Metadata.Model)
	}

	// Invalid requests are rejected client side, before any HTTP call.
	_, err = d.Decide(decide.NewRequest(context.Background(), mockDecideModel, decide.Text("x")))
	if err == nil || !strings.Contains(err.Error(), "at least one question") {
		t.Fatalf("expected validation error, got %v", err)
	}
}
