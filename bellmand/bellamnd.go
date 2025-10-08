package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/anthropic"
	"github.com/modfin/bellman/services/ollama"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/services/voyageai"
	"github.com/modfin/clix"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	slogchi "github.com/samber/slog-chi"
	"github.com/urfave/cli/v2"
)

var logger *slog.Logger

var instance = strings.ReplaceAll(uuid.New().String(), "-", "")[:16]

func main() {
	app := &cli.App{
		Name: "bellman wep api server",
		Flags: []cli.Flag{

			&cli.IntFlag{
				Name:    "http-port",
				EnvVars: []string{"BELLMAN_HTTP_PORT"},
				Value:   8080,
			},

			&cli.StringFlag{
				Name:    "log-format",
				EnvVars: []string{"BELLMAN_LOG_FORMAT"},
				Value:   "json",
				Usage:   "log format, json, text or color",
			},
			&cli.StringFlag{
				Name:    "log-level",
				EnvVars: []string{"BELLMAN_LOG_LEVEL"},
				Value:   "INFO",
				Usage:   "Levels are DEBUG, INFO, WARN, ERROR",
			},

			&cli.StringSliceFlag{
				Name:    "api-key",
				EnvVars: []string{"BELLMAN_API_KEY"},
			},

			&cli.StringFlag{
				Name:    "api-prefix",
				EnvVars: []string{"BELLMAN_API_PREFIX"},
			},

			&cli.StringFlag{
				Name:    "anthropic-key",
				EnvVars: []string{"BELLMAN_ANTHROPIC_KEY"},
			},

			&cli.StringFlag{
				Name:    "google-project",
				EnvVars: []string{"BELLMAN_GOOGLE_PROJECT"},
				Usage:   "The project which should be billed / it is executed in",
			},
			&cli.StringFlag{
				Name:    "google-region",
				EnvVars: []string{"BELLMAN_GOOGLE_REGION"},
				Usage:   "The region where the models are deployed, eg europe-north1",
			},
			&cli.StringFlag{
				Name:    "google-credential",
				EnvVars: []string{"BELLMAN_GOOGLE_CREDENTIAL"},
				Usage:   "Content of a service account key file, a json object. If not provided, default credentials will be used from environment. ie if its deployed on GCP",
			},
			&cli.StringFlag{
				Name:    "google-embed-models",
				EnvVars: []string{"BELLMAN_GOOGLE_EMBED_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "text-embedding-005"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
			},

			&cli.StringFlag{
				Name:    "openai-key",
				EnvVars: []string{"BELLMAN_OPENAI_KEY"},
			},

			&cli.StringFlag{
				Name:    "voyageai-key",
				EnvVars: []string{"BELLMAN_VOYAGEAI_KEY"},
			},

			&cli.StringFlag{
				Name:    "ollama-url",
				EnvVars: []string{"BELLMAN_OLLAMA_URL"},
				Usage:   `The url of the ollama service, eg http://localhost:11434`,
			},

			&cli.BoolFlag{
				Name:    "disable-gen-models",
				EnvVars: []string{"BELLMAN_DISABLE_GEN_MODELS"},
			},
			&cli.BoolFlag{
				Name:    "disable-embed-models",
				EnvVars: []string{"BELLMAN_DISABLE_EMBED_MODELS"},
			},

			&cli.StringFlag{
				Name:    "prometheus-metrics-basic-auth",
				EnvVars: []string{"BELLMAN_PROMETHEUS_METRICS_BASIC_AUTH"},
				Usage:   "protects /metrics endpoint, format is 'user:password'. /metrics not enabled if not set. No basic auth is just a colon, eg ':'",
			},
			&cli.StringFlag{
				Name:    "prometheus-push-url",
				EnvVars: []string{"BELLMAN_PROMETHEUS_PUSH_URL"},
				Usage:   "Use https://user:password@example.com to push metrics to prometheus push gateway",
			},
		},

		Action: func(context *cli.Context) error {
			setLogging(context)
			logger.Info("Start", "action", "parsing config")
			cfg := clix.Parse[Config](context)
			return serve(cfg)
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

func setLogging(ctx *cli.Context) {

	level := slog.Level(0)
	err := level.UnmarshalText([]byte(ctx.String("log-level")))
	if err != nil {
		panic(fmt.Errorf("could not parse log level, %w", err))
	}

	switch ctx.String("log-format") {
	case "color":
		slog.SetDefault(slog.New(
			tint.NewHandler(os.Stdout, &tint.Options{
				Level:      level,
				TimeFormat: time.DateTime,
			}),
		))
	case "text":
		fmt.Println("json")
		slog.SetDefault(slog.New(
			slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
				Level: level,
			})))
	default:
		fmt.Println("json")
		slog.SetDefault(slog.New(
			slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
				Level: level,
			})))
	}
	logger = slog.Default().With("instance", instance)
}

func httpErr(w http.ResponseWriter, err error, code int) {
	type errResp struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(errResp{Error: err.Error()})
}

type GoogleConfig struct {
	Credentials string `cli:"google-credential"`
	Project     string `cli:"google-project"`
	Region      string `cli:"google-region"`
}

type Config struct {
	ApiKeys   []string `cli:"api-key"`
	ApiPrefix string   `cli:"api-prefix"`

	HttpPort int `cli:"http-port"`

	DisableGenModels   bool `cli:"disable-gen-models"`
	DisableEmbedModels bool `cli:"disable-embed-models"`

	AnthropicKey string `cli:"anthropic-key"`
	OpenAiKey    string `cli:"openai-key"`
	Google       GoogleConfig
	VoyageAiKey  string `cli:"voyageai-key"`
	OllamaURL    string `cli:"ollama-url"`

	PrometheusMetricsBasicAuth string `cli:"prometheus-metrics-basic-auth"`
	PrometheusPushUrl          string `cli:"prometheus-push-url"`
}

func auth(cfg Config) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")

			header = strings.TrimPrefix(header, "Bearer ")

			if header == "" {
				httpErr(w, fmt.Errorf("missing authorization header"), http.StatusUnauthorized)
				return
			}

			if len(cfg.ApiKeys) == 0 {
				httpErr(w, fmt.Errorf("no api keys configured"), http.StatusUnauthorized)
				return
			}

			name, key, found := strings.Cut(header, "_")

			if !found {
				httpErr(w, fmt.Errorf("invalid authorization header, expected format {name}_{key}"), http.StatusUnauthorized)
				return
			}
			if !slices.Contains(cfg.ApiKeys, key) { // contant compare?
				time.Sleep(time.Duration(rand.Int63n(300)) * time.Millisecond)
				httpErr(w, fmt.Errorf("invalid api key"), http.StatusUnauthorized)
				return
			}

			r = r.WithContext(context.WithValue(r.Context(), "api-key-name", name))

			next.ServeHTTP(w, r)
		}

		return http.HandlerFunc(fn)
	}
}

func metricsAuth(userpass string) func(http.Handler) http.Handler {

	active := len(userpass) > 0
	open := userpass == ":"
	user, pass, _ := strings.Cut(userpass, ":")

	if !active {
		logger.Info("/metrics endpoint is disabled")
	}

	if open {
		logger.Warn("/metrics endpoint is open and has no protection")
	}

	if active && !open {
		logger.Info("/metrics endpoint is protected")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			if !active {
				http.NotFound(w, r)
				return
			}
			if open {
				next.ServeHTTP(w, r)
				return
			}

			auth := r.Header.Get("Authorization")
			if auth == "" {
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			_type, pwd, found := strings.Cut(auth, " ")
			if !found || _type != "Basic" {
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			payload, err := base64.StdEncoding.DecodeString(pwd)
			if err != nil {
				w.Header().Set("WWW-Authenticate", `Basic realm="restricted"`)
				http.Error(w, "Unauthorized, bad header", http.StatusUnauthorized)
				return
			}
			tryuser, trypass, found := strings.Cut(string(payload), ":")

			if !found || tryuser != user || trypass != pass {
				time.Sleep(time.Duration(rand.Int63n(300)) * time.Millisecond)
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func serve(cfg Config) error {
	var err error
	logger.Info("Start", "action", "setting up ai proxy")
	proxy, err := setupProxy(cfg)
	if err != nil {
		return fmt.Errorf("could not setup proxy, %w", err)
	}

	h := chi.NewRouter()

	r := func() *chi.Mux {
		apiPrefix := strings.TrimSpace(cfg.ApiPrefix)
		if apiPrefix == "" {
			return h
		}
		r := chi.NewRouter()
		h.Mount(apiPrefix, r)
		logger.Info("Start", "using api-prefix", apiPrefix)
		return r
	}()

	r.Use(middleware.Recoverer)
	r.Use(slogchi.New(logger))

	r.Handle("/metrics", metricsAuth(cfg.PrometheusMetricsBasicAuth)(promhttp.Handler()))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	if !cfg.DisableEmbedModels {
		r.Route("/embed", Embed(proxy, cfg))
	}
	if !cfg.DisableGenModels {
		r.Route("/gen", Gen(proxy, cfg))
	}

	server := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: h}
	go func() {
		logger.Info("Start", "action", "starting server", "port", cfg.HttpPort)
		err = server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			logger.Error("http server error", "err", err)
			os.Exit(1)
		}
	}()

	var pusher *PromPusher
	if cfg.PrometheusPushUrl != "" {
		pusher = &PromPusher{
			uri:     cfg.PrometheusPushUrl,
			stopped: make(chan struct{}),
			done:    make(chan struct{}),
		}
		go pusher.Start()
	}

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	term := <-sig
	logger.Info("Shutdown", "action", "got signal", "signal", term)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	logger.Info("Shutdown", "action", "shutting down http server")
	_ = server.Shutdown(ctx)

	if pusher != nil {
		logger.Info("Shutdown", "action", "shutting down prometheus pusher")
		_ = pusher.Stop(ctx)
	}
	logger.Info("Shutdown", "action", "termination complete")

	return nil
}

type PromPusher struct {
	uri     string
	stopped chan struct{}
	done    chan struct{}
}

func (p *PromPusher) Start() {
	var stopped bool

	for {
		if stopped {
			return
		}
		select {
		case <-p.stopped:
			stopped = true
		case <-time.After(30 * time.Second):
		}

		u, err := url.Parse(p.uri)
		if err != nil {
			logger.Error("[prometheus] could not parse prometheus url", "err", err)
			continue
		}

		user := u.User
		u.User = nil
		pusher := push.New(u.String(), "bellmand").
			Gatherer(prometheus.DefaultGatherer).
			Grouping("instance", instance)
		if user != nil {
			pass, _ := user.Password()
			pusher = pusher.BasicAuth(user.Username(), pass)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		logger.Info("[prometheus] pushing metrics")
		err = pusher.PushContext(ctx)
		cancel()
		if err != nil {
			logger.Error("[prometheus] could not push metrics to prometheus", "err", err)
		}
	}
}

func (p *PromPusher) Stop(ctx context.Context) error {
	close(p.stopped)
	select {
	case <-p.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func Gen(proxy *bellman.Proxy, cfg Config) func(r chi.Router) {

	var reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_gen_request_count",
			Help:        "Number of request per key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)

	var tokensCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_gen_token_count",
			Help:        "Number of token processed by model and key",
			ConstLabels: nil,
		},
		[]string{"model", "key", "type"},
	)

	var streamReqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_gen_stream_request_count",
			Help:        "Number of streaming request per key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)

	var streamTokensCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_gen_stream_token_count",
			Help:        "Number of token processed by model and key in streaming mode",
			ConstLabels: nil,
		},
		[]string{"model", "key", "type"},
	)
	prometheus.MustRegister(reqCounter, tokensCounter, streamReqCounter, streamTokensCounter)

	return func(r chi.Router) {
		r.Use(auth(cfg))

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {

			body, err := io.ReadAll(r.Body)
			if err != nil {
				err = fmt.Errorf("could not read request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}

			var req gen.FullRequest
			err = json.Unmarshal(body, &req)
			if err != nil {
				err = fmt.Errorf("could not decode request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}

			gen, err := proxy.Gen(req.Model)
			if err != nil {
				err = fmt.Errorf("could not get generator, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			gen = gen.SetConfig(req.Request).WithContext(r.Context())
			response, err := gen.Prompt(req.Prompts...)
			if err != nil {
				logger.Error("gen request", "err", err)
				err = fmt.Errorf("could not generate text, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("gen request",
				"key", keyName,
				"model", req.Model.FQN(),
				"token-input", response.Metadata.InputTokens,
				"token-output", response.Metadata.OutputTokens,
				"token-total", response.Metadata.TotalTokens,
			)

			// Taking some metrics...
			reqCounter.WithLabelValues(response.Metadata.Model, keyName.(string)).Inc()
			tokensCounter.WithLabelValues(response.Metadata.Model, keyName.(string), "total").Add(float64(response.Metadata.TotalTokens))
			tokensCounter.WithLabelValues(response.Metadata.Model, keyName.(string), "input").Add(float64(response.Metadata.InputTokens))
			tokensCounter.WithLabelValues(response.Metadata.Model, keyName.(string), "output").Add(float64(response.Metadata.OutputTokens))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)

		})

		r.Post("/stream", func(w http.ResponseWriter, r *http.Request) {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				err = fmt.Errorf("could not read request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}

			var req gen.FullRequest
			err = json.Unmarshal(body, &req)
			if err != nil {
				err = fmt.Errorf("could not decode request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}

			// Force streaming mode
			req.Stream = true

			gen, err := proxy.Gen(req.Model)
			if err != nil {
				err = fmt.Errorf("could not get generator, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			gen = gen.SetConfig(req.Request).WithContext(r.Context())

			// Get streaming response
			stream, err := gen.Stream(req.Prompts...)
			if err != nil {
				logger.Error("gen stream request", "err", err)
				err = fmt.Errorf("could not start streaming, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("gen stream request",
				"key", keyName,
				"model", req.Model.FQN(),
			)

			// Set SSE headers
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Headers", "Cache-Control")

			// Ensure the response is flushed immediately
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}

			// Track metrics
			var totalInputTokens, totalOutputTokens int
			var modelName string

			// Process streaming responses
			for streamResp := range stream {
				// Handle context cancellation
				select {
				case <-r.Context().Done():
					logger.Info("gen stream cancelled", "key", keyName, "model", req.Model.FQN())
					return
				default:
				}

				// Update metrics
				if streamResp.Metadata != nil {
					totalInputTokens = streamResp.Metadata.InputTokens
					totalOutputTokens = streamResp.Metadata.OutputTokens
					modelName = streamResp.Metadata.Model
				}

				// Convert to SSE format
				data, err := json.Marshal(streamResp)
				if err != nil {
					logger.Error("gen stream marshal error", "err", err)
					continue
				}

				// Write SSE event
				_, err = fmt.Fprintf(w, "data: %s\n\n", data)
				if err != nil {
					logger.Error("gen stream write error", "err", err)
					break
				}

				// Flush the response
				if flusher, ok := w.(http.Flusher); ok {
					flusher.Flush()
				}

				// Check for end of stream
				if streamResp.Type == "EOF" {
					break
				}
			}

			// Log final metrics
			logger.Info("gen stream completed",
				"key", keyName,
				"model", req.Model.FQN(),
				"token-input", totalInputTokens,
				"token-output", totalOutputTokens,
				"token-total", totalInputTokens+totalOutputTokens,
			)

			// Update metrics
			streamReqCounter.WithLabelValues(modelName, keyName.(string)).Inc()
			streamTokensCounter.WithLabelValues(modelName, keyName.(string), "total").Add(float64(totalInputTokens + totalOutputTokens))
			streamTokensCounter.WithLabelValues(modelName, keyName.(string), "input").Add(float64(totalInputTokens))
			streamTokensCounter.WithLabelValues(modelName, keyName.(string), "output").Add(float64(totalOutputTokens))
		})
	}
}

func Embed(proxy *bellman.Proxy, cfg Config) func(r chi.Router) {

	var reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_embed_request_count",
			Help:        "Number of request per key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)

	var tokensCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "bellman_embed_token_count",
			Help:        "Number of token processed by model and key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)
	prometheus.MustRegister(reqCounter, tokensCounter)

	return func(r chi.Router) {
		r.Use(auth(cfg))

		//r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
		//	models := proxy.EmbedModels()
		//	w.Header().Set("Content-Type", "application/json")
		//	_ = json.NewEncoder(w).Encode(models)
		//})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req embed.Request
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				err = fmt.Errorf("could not decode request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}
			req.Ctx = r.Context()

			response, err := proxy.Embed(&req)
			if err != nil {
				err = fmt.Errorf("could not embed text, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("embed request",
				"key", keyName,
				"model", req.Model.FQN(),
				"texts", len(req.Texts),
				"token-total", response.Metadata.TotalTokens,
			)

			// Taking some metrics...
			reqCounter.WithLabelValues(response.Metadata.Model, keyName.(string)).Inc()
			tokensCounter.WithLabelValues(response.Metadata.Model, keyName.(string)).Add(float64(response.Metadata.TotalTokens))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)

		})

		r.Post("/document", func(w http.ResponseWriter, r *http.Request) {
			var req embed.DocumentRequest
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				err = fmt.Errorf("could not decode request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}
			req.Ctx = r.Context()

			response, err := proxy.EmbedDocument(&req)
			if err != nil {
				err = fmt.Errorf("could not embed text, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("embed document request",
				"key", keyName,
				"model", req.Model.FQN(),
				"chunks", len(req.DocumentChunks),
				"token-total", response.Metadata.TotalTokens,
			)

			// Taking some metrics...
			reqCounter.WithLabelValues(response.Metadata.Model, keyName.(string)).Inc()
			tokensCounter.WithLabelValues(response.Metadata.Model, keyName.(string)).Add(float64(response.Metadata.TotalTokens))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(response)

		})
	}
}

func setupProxy(cfg Config) (*bellman.Proxy, error) {

	proxy := bellman.NewProxy()

	if cfg.AnthropicKey != "" {
		client := anthropic.New(cfg.AnthropicKey)

		proxy.RegisterGen(client)

		logger.Info("Start", "action", "[gen] adding provider", "provider", client.Provider())

	}
	if cfg.OpenAiKey != "" {
		client := openai.New(cfg.OpenAiKey)

		proxy.RegisterGen(client)
		proxy.RegisterEmbeder(client)
		logger.Info("Start", "action", "[gen] adding provider", "provider", client.Provider())
		logger.Info("Start", "action", "[embed] adding  provider", "provider", client.Provider())
	}

	if cfg.Google.Region != "" && cfg.Google.Project != "" {
		var err error
		client, err := vertexai.New(vertexai.GoogleConfig{
			Project:    cfg.Google.Project,
			Region:     cfg.Google.Region,
			Credential: cfg.Google.Credentials,
		})
		if err != nil {
			return nil, err
		}

		proxy.RegisterGen(client)
		proxy.RegisterEmbeder(client)
		logger.Info("Start", "action", "[gen] adding provider", "provider", client.Provider())
		logger.Info("Start", "action", "[embed] adding  provider", "provider", client.Provider())
	}

	if cfg.VoyageAiKey != "" {
		client := voyageai.New(cfg.VoyageAiKey)
		proxy.RegisterEmbeder(client)
		logger.Info("Start", "action", "[embed] adding  provider", "provider", client.Provider())
	}

	if cfg.OllamaURL != "" {
		client := ollama.New(cfg.OllamaURL)

		proxy.RegisterGen(client)
		proxy.RegisterEmbeder(client)
		logger.Info("Start", "action", "[embed] adding  provider", "provider", client.Provider())
	}

	return proxy, nil
}
