package bellman_test

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
)

func TestStreamDoesNotStrandConnections(t *testing.T) {
	for _, done := range []bool{true, false} {
		t.Run(fmt.Sprintf("done=%v", done), func(t *testing.T) {
			var mu sync.Mutex
			open := map[net.Conn]bool{}
			srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				fmt.Fprint(w, "data: {\"type\":\"delta\",\"role\":\"assistant\",\"content\":\"hi\"}\n\n")
				if done {
					fmt.Fprint(w, "data: [DONE]\n\n")
				}
			}))
			srv.Config.ConnState = func(c net.Conn, s http.ConnState) {
				mu.Lock()
				defer mu.Unlock()
				switch s {
				case http.StateNew:
					open[c] = true
				case http.StateClosed, http.StateHijacked:
					delete(open, c)
				}
			}
			srv.Start()
			defer srv.Close()

			b := bellman.New(srv.URL, bellman.Key{Name: "test", Token: "k"})
			const streams = 50
			for i := 0; i < streams; i++ {
				ch, err := b.Generator().Model(gen.Model{Provider: "VertexAI", Name: "m"}).Stream(prompt.AsUser("hello"))
				if err != nil {
					t.Fatal(err)
				}
				for range ch {
				}
			}
			time.Sleep(200 * time.Millisecond)
			mu.Lock()
			n := len(open)
			mu.Unlock()
			if n > 4 {
				t.Fatalf("%d of %d finished streams left a connection open", n, streams)
			}
		})
	}
}
