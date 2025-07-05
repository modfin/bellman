package gen

import (
   "context"
   "errors"
   "testing"

   "github.com/modfin/bellman/models"
   "github.com/modfin/bellman/tools"
)

func TestStreamResponseError(t *testing.T) {
   sr := StreamResponse{Type: TYPE_ERROR, Content: "oops"}
   err := sr.Error()
   want := "streaming response error: oops"
   if err == nil || err.Error() != want {
       t.Errorf("Error() = %v, want %q", err, want)
   }
   sr2 := StreamResponse{Type: TYPE_DELTA, Content: "data"}
   if e := sr2.Error(); e != nil {
       t.Errorf("Error() for non-error type = %v, want nil", e)
   }
}

func TestResponseTextAndTools(t *testing.T) {
   r := &Response{Texts: []string{"t1"}}
   if !r.IsText() {
       t.Error("IsText() = false, want true")
   }
   if r.IsTools() {
       t.Error("IsTools() = true, want false")
   }
   text, err := r.AsText()
   if err != nil || text != "t1" {
       t.Errorf("AsText() = %q, %v, want %q, nil", text, err, "t1")
   }
   if _, err := r.AsTools(); err == nil {
       t.Error("AsTools() = nil, want error")
   }

   // Tools only
   tool := tools.Tool{Name: "x", Function: func(context.Context, tools.Call) (string, error) { return "", nil }}
   r2 := &Response{Tools: []tools.Call{{Name: "x", Ref: &tool}}}
   if !r2.IsTools() {
       t.Error("IsTools() = false, want true")
   }
   if r2.IsText() {
       t.Error("IsText() = true, want false")
   }
   calls, err := r2.AsTools()
   if err != nil {
       t.Errorf("AsTools() unexpected error: %v", err)
   }
   if len(calls) != 1 || calls[0].Name != "x" {
       t.Errorf("AsTools() = %v, want call x", calls)
   }
}

func TestResponseUnmarshal(t *testing.T) {
   var obj struct{ Foo string }
   r := &Response{Texts: []string{`{"Foo":"bar"}`}}
   if err := r.Unmarshal(&obj); err != nil {
       t.Errorf("Unmarshal() error: %v", err)
   }
   if obj.Foo != "bar" {
       t.Errorf("Unmarshal wrote %+v, want Foo=\"bar\"", obj)
   }
   r2 := &Response{Texts: []string{"invalid"}}
   if err := r2.Unmarshal(&obj); err == nil {
       t.Error("Unmarshal() = nil, want error for invalid JSON")
   }
}

func TestResponseEval(t *testing.T) {
   // missing Ref
   r := &Response{Tools: []tools.Call{{Name: "t1"}}}
   if err := r.Eval(context.Background()); err == nil {
       t.Error("Eval() = nil, want error for missing Ref")
   }
   // missing Function
   tool2 := tools.Tool{Name: "t2"}
   r2 := &Response{Tools: []tools.Call{{Name: "t2", Ref: &tool2}}}
   if err := r2.Eval(context.Background()); err == nil {
       t.Error("Eval() = nil, want error for missing Function")
   }
   // callback error
   fn := func(ctx context.Context, call tools.Call) (string, error) {
       return "", errors.New("fail")
   }
   tool3 := tools.Tool{Name: "t3", Function: fn}
   r3 := &Response{Tools: []tools.Call{{Name: "t3", Ref: &tool3}}}
   if err := r3.Eval(context.Background()); err == nil {
       t.Error("Eval() = nil, want error for callback failure")
   }
   // success
   fn2 := func(ctx context.Context, call tools.Call) (string, error) { return "ok", nil }
   tool4 := tools.Tool{Name: "t4", Function: fn2}
   r4 := &Response{Tools: []tools.Call{{Name: "t4", Ref: &tool4}}, Metadata: models.Metadata{}}
   if err := r4.Eval(context.Background()); err != nil {
       t.Errorf("Eval() error: %v", err)
   }
}