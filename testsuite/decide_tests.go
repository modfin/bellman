package testsuite

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"

	"github.com/modfin/bellman/models/decide"
)

const decideRefundTicket = "Hi, I was charged twice for my order #4411 last week and I need the extra charge refunded today. This is urgent, I am losing money."
const decideQuietTicket = "Hello, just wondering what colours the hoodie comes in. No rush, thanks!"

func routeQuestion() decide.Question {
	return decide.Choice("department", "Which team should handle this message?",
		decide.NewOption("billing", "Charges, invoices, refunds and payment problems."),
		decide.NewOption("shipping", "Delivery status, delays and lost packages."),
		decide.NewOption("product", "Questions about products, sizes and colours."),
		decide.NewOption("other", "Anything that does not fit the other options."),
	)
}

func urgencyQuestion() decide.Question {
	return decide.Score("urgency", "How urgent is this message?",
		decide.NewLevel("low", "No deadline, the customer is browsing or curious."),
		decide.NewLevel("medium", "The customer wants something done but there is no deadline today."),
		decide.NewLevel("high", "The customer is losing money or has a deadline today."),
	)
}

func checkDistribution(t tester, a decide.Answer, keys []string) {
	if len(a.Probabilities) != len(keys) {
		t.Fatalf("answer %q: expected %d probabilities, got %d (%v)", a.Name, len(keys), len(a.Probabilities), a.Probabilities)
	}
	var sum float64
	for _, k := range keys {
		p, ok := a.Probabilities[k]
		if !ok {
			t.Fatalf("answer %q: missing probability for %q (%v)", a.Name, k, a.Probabilities)
		}
		if p < 0 || p > 1 {
			t.Fatalf("answer %q: probability for %q out of range: %v", a.Name, k, p)
		}
		sum += p
	}
	if math.Abs(sum-1) > 0.05 {
		t.Fatalf("answer %q: probabilities sum to %v, expected ~1 (%v)", a.Name, sum, a.Probabilities)
	}
	if a.Confidence < 0 || a.Confidence > 1 {
		t.Fatalf("answer %q: confidence out of range: %v", a.Name, a.Confidence)
	}
}

func testDecidePredicate(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		q := decide.Predicate("refund", "Is the customer asking for money back?").
			WithCriteria("The customer wants a charge reversed or money returned.", "No refund is requested.")

		res, err := d.Decide(decide.NewRequest(context.Background(), m, decide.Text(decideRefundTicket)).Ask(q))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		p, err := res.Probability("refund")
		if err != nil {
			t.Fatalf("Probability() error = %v", err)
		}
		if p < 0.5 {
			t.Fatalf("expected P(refund) > 0.5 for a refund request, got %v", p)
		}

		res, err = d.Decide(decide.NewRequest(context.Background(), m, decide.Text(decideQuietTicket)).Ask(q))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		p, err = res.Probability("refund")
		if err != nil {
			t.Fatalf("Probability() error = %v", err)
		}
		if p > 0.5 {
			t.Fatalf("expected P(refund) < 0.5 for a product question, got %v", p)
		}
		if res.Metadata.Model != m.FQN() {
			t.Fatalf("expected metadata model %q, got %q", m.FQN(), res.Metadata.Model)
		}
	}
}

func testDecideChoice(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		res, err := d.Decide(decide.NewRequest(context.Background(), m, decide.Text(decideRefundTicket)).Ask(routeQuestion()))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		a, ok := res.Answer("department")
		if !ok {
			t.Fatalf("no answer for department")
		}
		if a.Choice != "billing" {
			t.Fatalf("expected choice billing, got %q (%v)", a.Choice, a.Probabilities)
		}
		checkDistribution(t, a, []string{"billing", "shipping", "product", "other"})
		for k, p := range a.Probabilities {
			if p > a.Probabilities[a.Choice] {
				t.Fatalf("choice %q is not the argmax, %q has %v", a.Choice, k, p)
			}
		}
	}
}

func testDecideScore(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		res, err := d.Decide(decide.NewRequest(context.Background(), m, decide.Text(decideRefundTicket)).Ask(urgencyQuestion()))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		a, ok := res.Answer("urgency")
		if !ok {
			t.Fatalf("no answer for urgency")
		}
		checkDistribution(t, a, []string{"0", "1", "2"})
		if a.Score < 0 || a.Score > 2 {
			t.Fatalf("score out of range [0,2]: %v", a.Score)
		}
		if a.Score < 1 {
			t.Fatalf("expected an urgent ticket to score >= 1, got %v (%v)", a.Score, a.Probabilities)
		}
		if a.Level() < 1 {
			t.Fatalf("expected Level() >= 1, got %d", a.Level())
		}
	}
}

func testDecideMixed(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		req := decide.NewRequest(context.Background(), m, decide.Text(decideRefundTicket)).Ask(
			decide.Predicate("refund", "Is the customer asking for money back?"),
			routeQuestion(),
			urgencyQuestion(),
		)
		res, err := d.Decide(req)
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		if len(res.Answers) != 3 {
			t.Fatalf("expected 3 answers, got %d", len(res.Answers))
		}
		for _, name := range []string{"refund", "department", "urgency"} {
			a, ok := res.Answer(name)
			if !ok {
				t.Fatalf("missing answer %q", name)
			}
			if a.Refused {
				t.Fatalf("answer %q was refused", name)
			}
			if a.Name != name {
				t.Fatalf("answer keyed %q has name %q", name, a.Name)
			}
		}
		if res.Metadata.TotalTokens == 0 && res.Metadata.InputTokens == 0 {
			t.Fatalf("expected token usage to be reported")
		}
	}
}

func testDecideJSONState(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		state := map[string]any{
			"order": map[string]any{"id": "A-1", "total": 42.5, "status": "refunded"},
			"agent": "bot-7",
		}
		res, err := d.Decide(decide.NewRequest(context.Background(), m, decide.JSON(state)).Ask(
			decide.Predicate("refunded", "Has the order been refunded?"),
		))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		p, err := res.Probability("refunded")
		if err != nil {
			t.Fatalf("Probability() error = %v", err)
		}
		if p < 0.5 {
			t.Fatalf("expected P(refunded) > 0.5 for a refunded order, got %v", p)
		}
	}
}

func testDecideImage(d decide.Decider, m decide.Model) func(tester) {
	return func(t tester) {
		img := image.NewRGBA(image.Rect(0, 0, 64, 64))
		for y := 0; y < 64; y++ {
			for x := 0; x < 64; x++ {
				img.Set(x, y, color.RGBA{R: 220, G: 20, B: 20, A: 255})
			}
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			t.Fatalf("png encode: %v", err)
		}

		res, err := d.Decide(decide.NewRequest(context.Background(), m,
			decide.Text("Look at the attached image."),
			decide.Image("image/png", buf.Bytes()),
		).Ask(
			decide.Choice("colour", "What is the dominant colour of the image?",
				decide.NewOption("red", ""),
				decide.NewOption("green", ""),
				decide.NewOption("blue", ""),
			),
		))
		if err != nil {
			t.Fatalf("Decide() error = %v", err)
		}
		c, err := res.Choice("colour")
		if err != nil {
			t.Fatalf("Choice() error = %v", err)
		}
		if c != "red" {
			t.Fatalf("expected red, got %q", c)
		}
	}
}
