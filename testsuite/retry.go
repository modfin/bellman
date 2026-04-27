package testsuite

import (
	"fmt"
	"testing"
)

// tester is the subset of testing.TB the inner test bodies need. Decoupling
// from *testing.T lets withRetry rerun a body without each failed attempt
// poisoning the parent test — t.Run subtests would propagate failures.
type tester interface {
	Fatalf(format string, args ...any)
	Logf(format string, args ...any)
	Helper()
}

type fatalSignal struct{ msg string }

type retryT struct {
	t      *testing.T
	failed bool
	msg    string
}

func (r *retryT) Fatalf(format string, args ...any) {
	r.failed = true
	r.msg = fmt.Sprintf(format, args...)
	panic(fatalSignal{msg: r.msg})
}

func (r *retryT) Logf(format string, args ...any) {
	r.t.Logf(format, args...)
}

func (r *retryT) Helper() {
	r.t.Helper()
}

// withRetry runs fn up to attempts times against a fresh retryT. The first
// passing attempt wins; if all fail, the real *testing.T is failed with the
// last error message. Integration tests against real LLMs are inherently
// flaky (sampling jitter, regional buffering), so a small retry budget cuts
// false negatives without masking real regressions.
func withRetry(t *testing.T, attempts int, fn func(tester)) {
	t.Helper()
	var lastMsg string
	for i := 0; i < attempts; i++ {
		rt := &retryT{t: t}
		passed := func() (ok bool) {
			defer func() {
				if r := recover(); r != nil {
					if _, isFatal := r.(fatalSignal); isFatal {
						ok = false
						return
					}
					panic(r)
				}
			}()
			fn(rt)
			return !rt.failed
		}()
		if passed {
			if i > 0 {
				t.Logf("passed on attempt %d/%d", i+1, attempts)
			}
			return
		}
		lastMsg = rt.msg
		if i < attempts-1 {
			t.Logf("attempt %d/%d failed: %s — retrying", i+1, attempts, lastMsg)
		}
	}
	t.Fatalf("all %d attempts failed; last error: %s", attempts, lastMsg)
}
