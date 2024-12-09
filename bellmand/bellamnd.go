package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/lmittmann/tint"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/services/anthropic"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/services/voyageai"
	"github.com/modfin/clix"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/client_golang/prometheus/push"
	slogchi "github.com/samber/slog-chi"
	"github.com/urfave/cli/v2"
	"io"
	"log"
	"log/slog"
	"maps"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"
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
				Name:    "anthropic-key",
				EnvVars: []string{"BELLMAN_ANTHROPIC_KEY"},
			},
			&cli.StringFlag{
				Name:    "anthropic-gen-models",
				EnvVars: []string{"BELLMAN_ANTHROPIC_GEN_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "claude-3-5-haiku-latest"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
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
				Name:    "google-gen-models",
				EnvVars: []string{"BELLMAN_GOOGLE_GEN_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "gemini-1.5-flash-002"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
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
				Name:    "openai-gen-models",
				EnvVars: []string{"BELLMAN_OPENAI_GEN_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "chatgpt-4o-latest"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
			},
			&cli.StringFlag{
				Name:    "openai-embed-models",
				EnvVars: []string{"BELLMAN_OPENAI_EMBED_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "text-embedding-ada-002"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
			},

			&cli.StringFlag{
				Name:    "voyageai-key",
				EnvVars: []string{"BELLMAN_VOYAGEAI_KEY"},
			},
			&cli.StringFlag{
				Name:    "voyageai-embed-models",
				EnvVars: []string{"BELLMAN_VOYAGEAI_EMBED_MODELS"},
				Usage: `A json array containing objects with the name of the model, 
	eg [{"name": "voyage-3-lite"}]. If not provided, all default models will be loaded. 
	If provided, only the models in the array will be loaded.`,
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
	//logger.Error("Error Test")
	//logger.Warn("Warm Test")
	//logger.Info("Info Test")
	//logger.Debug("Debug Test")
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
	ApiKeys []string `cli:"api-key"`

	HttpPort int `cli:"http-port"`

	AnthropicKey       string `cli:"anthropic-key"`
	AnthropicGenModels string `cli:"anthropic-gen-models"`

	OpenAiKey         string `cli:"openai-key"`
	OpenAiGenModels   string `cli:"openai-gen-models"`
	OpenAiEmbedModels string `cli:"openai-embed-models"`

	Google            GoogleConfig
	GoogleGenModels   string `cli:"google-gen-models"`
	GoogleEmbedModels string `cli:"google-embed-models"`

	VoyageAiKey         string `cli:"voyageai-key"`
	VoyageAiEmbedModels string `cli:"voyageai-embed-models"`

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

	logger.Info("Start", "action", "setting up ai proxy")
	proxy, err := setupProxy(cfg)
	if err != nil {
		return fmt.Errorf("could not setup proxy, %w", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(slogchi.New(logger))

	r.Handle("/metrics", metricsAuth(cfg.PrometheusMetricsBasicAuth)(promhttp.Handler()))

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	r.Route("/gen", Gen(proxy, cfg))
	r.Route("/embed", Embed(proxy, cfg))

	server := &http.Server{Addr: fmt.Sprintf(":%d", cfg.HttpPort), Handler: r}
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
			Name:        "gen_request_count",
			Help:        "Number of request per key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)

	var tokensCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "gen_token_count",
			Help:        "Number of token processed by model and key",
			ConstLabels: nil,
		},
		[]string{"model", "key", "type"},
	)
	prometheus.MustRegister(reqCounter, tokensCounter)

	return func(r chi.Router) {
		r.Use(auth(cfg))

		r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
			models := proxy.GenModels()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models)
		})

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

			gen = gen.SetConfig(req.Request)
			response, err := gen.Prompt(req.Prompts...)
			if err != nil {
				err = fmt.Errorf("could not generate text, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("gen request",
				"model", req.Model,
				"key", keyName,
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

	}
}

func Embed(proxy *bellman.Proxy, cfg Config) func(r chi.Router) {

	var reqCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "embed_request_count",
			Help:        "Number of request per key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)

	var tokensCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name:        "embed_token_count",
			Help:        "Number of token processed by model and key",
			ConstLabels: nil,
		},
		[]string{"model", "key"},
	)
	prometheus.MustRegister(reqCounter, tokensCounter)

	return func(r chi.Router) {
		r.Use(auth(cfg))

		r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
			models := proxy.EmbedModels()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models)
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req embed.Request
			err := json.NewDecoder(r.Body).Decode(&req)
			if err != nil {
				err = fmt.Errorf("could not decode request, %w", err)
				httpErr(w, err, http.StatusBadRequest)
				return
			}

			response, err := proxy.Embed(req)
			if err != nil {
				err = fmt.Errorf("could not embed text, %w", err)
				httpErr(w, err, http.StatusInternalServerError)
				return
			}

			keyName := r.Context().Value("api-key-name")
			logger.Info("embed request",
				"model", req.Model,
				"key", keyName,
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

func genModelsOf(str string, provider string) ([]gen.Model, error) {
	var models []gen.Model
	err := json.Unmarshal([]byte(str), &models)
	if err != nil {
		return nil, fmt.Errorf("could not parse gen models, %w", err)
	}
	for i, _ := range models {
		models[i].Provider = provider
	}
	return models, nil
}
func embedModelsOf(str string, provider string) ([]embed.Model, error) {
	var models []embed.Model
	err := json.Unmarshal([]byte(str), &models)
	if err != nil {
		return nil, fmt.Errorf("could not parse gen models, %w", err)
	}
	for i, _ := range models {
		models[i].Provider = provider
	}
	return models, nil
}

func setupProxy(cfg Config) (*bellman.Proxy, error) {
	var err error

	proxy := bellman.NewProxy()

	if cfg.AnthropicKey != "" {
		logger.Info("Start", "action", "proxy: adding Anthropic models")

		client := anthropic.New(cfg.AnthropicKey)

		genModels := slices.Collect(maps.Values(anthropic.GenModels))
		if len(cfg.AnthropicGenModels) > 0 {
			genModels, err = genModelsOf(cfg.AnthropicGenModels, anthropic.Provider)
			if err != nil {
				return nil, fmt.Errorf("could not get gen models, %w", err)
			}
		}
		proxy.RegisterGen(client, genModels...)

		for _, model := range genModels {
			logger.Info("Start", "action", "proxy: adding model [gen]", "model", model.FQN())
		}
	}
	if cfg.OpenAiKey != "" {
		client := openai.New(cfg.OpenAiKey)

		genModels := slices.Collect(maps.Values(openai.GenModels))
		if len(cfg.OpenAiGenModels) > 0 {
			genModels, err = genModelsOf(cfg.OpenAiGenModels, openai.Provider)
			if err != nil {
				return nil, fmt.Errorf("could not get gen models, %w", err)
			}
		}

		proxy.RegisterGen(client, genModels...)
		proxy.RegisterEmbeder(client, slices.Collect(maps.Values(openai.EmbedModels))...)

		for _, model := range genModels {
			logger.Info("Start", "action", "proxy: adding model [gen]", "model", model.FQN())
		}
		for _, model := range openai.EmbedModels {
			logger.Info("Start", "action", "proxy: adding model [embed]", "model", model.FQN())
		}
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

		genModels := slices.Collect(maps.Values(vertexai.GenModels))
		if len(cfg.GoogleGenModels) > 0 {
			genModels, err = genModelsOf(cfg.GoogleGenModels, vertexai.Provider)
			if err != nil {
				return nil, fmt.Errorf("could not get gen models, %w", err)
			}
		}
		proxy.RegisterGen(client, genModels...)
		proxy.RegisterEmbeder(client, slices.Collect(maps.Values(vertexai.EmbedModels))...)

		for _, model := range genModels {
			logger.Info("Start", "action", "proxy: adding model [gen]", "model", model.FQN())
		}
		for _, model := range vertexai.EmbedModels {
			logger.Info("Start", "action", "proxy: adding model [embed]", "model", model.FQN())
		}
	}

	if cfg.VoyageAiKey != "" {
		client := voyageai.New(cfg.VoyageAiKey)

		embedModels := slices.Collect(maps.Values(voyageai.EmbedModels))
		if len(cfg.VoyageAiEmbedModels) > 0 {
			embedModels, err = embedModelsOf(cfg.VoyageAiEmbedModels, voyageai.Provider)
			if err != nil {
				return nil, fmt.Errorf("could not get gen models, %w", err)
			}
		}

		proxy.RegisterEmbeder(client, embedModels...)

		for _, model := range embedModels {
			logger.Info("Start", "action", "proxy: adding model [embed]", "model", model.FQN())
		}

	}

	return proxy, nil
}
