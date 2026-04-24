package main_test

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"

	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/testsuite"
)

const (
	testAPIKey     = "integration-test-key"
	testAPIKeyName = "itest"
)

// TestBellmandIntegration exercises the bellmand HTTP boundary end-to-end.
// Per-provider semantics are covered by the service-level integration tests;
// this one only needs a single provider to validate auth, request marshaling,
// streaming, and embeddings over HTTP. OpenAI covers all three capabilities.
func TestBellmandIntegration(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY not set")
	}

	bin := buildBinary(t)
	url := startServer(t, bin)
	c := bellman.New(url, bellman.Key{Name: testAPIKeyName, Token: testAPIKey})

	testsuite.Run(t, c.Generator(gen.WithModel(openai.GenModel_gpt5_4_mini_latest)),
		testsuite.Capabilities{
			Tools:               true,
			StructuredOutput:    true,
			Streaming:           true,
			Agent:               true,
			StreamThinkingTools: true,
		})
	testsuite.RunEmbed(t, c, openai.EmbedModel_text3_small,
		testsuite.EmbedCapabilities{Single: true, Many: true})
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bellmand")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("go build bellmand: %v", err)
	}
	return bin
}

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startServer(t *testing.T, bin string) string {
	t.Helper()
	port := freePort(t)
	internalPort := freePort(t)

	cmd := exec.Command(bin)
	cmd.Env = append(os.Environ(),
		"BELLMAN_HTTP_PORT="+strconv.Itoa(port),
		"BELLMAN_INTERNAL_HTTP_PORT="+strconv.Itoa(internalPort),
		"BELLMAN_API_KEY="+testAPIKey,
		"BELLMAN_LOG_FORMAT=text",
		"BELLMAN_LOG_LEVEL=WARN",
	)
	cmd.Env = append(cmd.Env, "BELLMAN_OPENAI_KEY="+os.Getenv("OPENAI_API_KEY"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start bellmand: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	if err := waitHealthy(url, 30*time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		t.Fatalf("bellmand never became healthy: %v", err)
	}

	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()
		select {
		case <-done:
		case <-time.After(12 * time.Second):
			_ = cmd.Process.Kill()
			<-done
		}
	})

	return url
}

func waitHealthy(base string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/health")
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("health check timed out after %s", timeout)
}
