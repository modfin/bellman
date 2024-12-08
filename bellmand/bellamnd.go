package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
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
	"github.com/urfave/cli/v2"
	"log"
	"log/slog"
	"maps"
	"math/rand"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"
)

var logger = slog.Default()

func main() {
	app := &cli.App{
		Name: "bellman wep api server",
		Flags: []cli.Flag{

			&cli.IntFlag{
				Name:    "http-port",
				EnvVars: []string{"BELLMAN_HTTP_PORT"},
				Value:   8080,
			},

			&cli.StringSliceFlag{
				Name:    "api-key",
				EnvVars: []string{"BELLMAN_API_KEY"},
			},

			&cli.StringFlag{
				Name:    "google-project",
				EnvVars: []string{"BELLMAN_GOOGLE_PROJECT"},
			},
			&cli.StringFlag{
				Name:    "google-region",
				EnvVars: []string{"BELLMAN_GOOGLE_REGION"},
			},
			&cli.StringFlag{
				Name:    "google-credential",
				EnvVars: []string{"BELLMAN_GOOGLE_CREDENTIAL"},
			},

			&cli.StringFlag{
				Name:    "anthropic-key",
				EnvVars: []string{"BELLMAN_ANTHROPIC_KEY"},
			},
			&cli.StringFlag{
				Name:    "openai-key",
				EnvVars: []string{"BELLMAN_OPENAI_KEY"},
			},
			&cli.StringFlag{
				Name:    "voyageai-key",
				EnvVars: []string{"BELLMAN_VOYAGEAI_KEY"},
			},
		},
		Action: func(context *cli.Context) error {

			logger.Info("Start", "action", "parsing config")
			cfg := clix.Parse[Config](context)
			return serve(cfg)
		},
	}
	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
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

	AnthropicKey string `cli:"anthropic-key"`
	OpenAiKey    string `cli:"openai-key"`
	VoyageAiKey  string `cli:"voyageai-key"`

	Google GoogleConfig
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

func serve(cfg Config) error {

	logger.Info("Start", "action", "setting up ai proxy")
	proxy, err := setupProxy(cfg)
	if err != nil {
		return fmt.Errorf("could not setup proxy, %w", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(auth(cfg))
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/gen", Gen(proxy))
	r.Route("/embed", Embed(proxy))

	err = http.ListenAndServe(fmt.Sprintf(":%d", cfg.HttpPort), r)

	if err != nil {
		return fmt.Errorf("could not start server, %w", err)
	}
	return nil
}

func Gen(proxy *bellman.Proxy) func(r chi.Router) {

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
		r.Get("/models", func(w http.ResponseWriter, r *http.Request) {
			models := proxy.GenModels()
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(models)
		})

		r.Post("/", func(w http.ResponseWriter, r *http.Request) {
			var req gen.FullRequest
			err := json.NewDecoder(r.Body).Decode(&req)
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

			// Taking some metrics...
			keyName := r.Context().Value("api-key-name")
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

func Embed(proxy *bellman.Proxy) func(r chi.Router) {

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

			// Taking some metrics...
			keyName := r.Context().Value("api-key-name")
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
		logger.Info("Start", "action", "proxy: adding Anthropic models")

		client := anthropic.New(cfg.AnthropicKey)
		proxy.RegisterGen(client, slices.Collect(maps.Values(anthropic.GenModels))...)

		for _, model := range anthropic.GenModels {
			logger.Info("Start", "action", "proxy: adding gen model", "model", model.FQN())
		}
	}
	if cfg.OpenAiKey != "" {

		client := openai.New(cfg.OpenAiKey)
		proxy.RegisterGen(client, slices.Collect(maps.Values(openai.GenModels))...)
		proxy.RegisterEmbeder(client, slices.Collect(maps.Values(openai.EmbedModels))...)

		for _, model := range openai.GenModels {
			logger.Info("Start", "action", "proxy: adding gen model", "model", model.FQN())
		}
		for _, model := range openai.EmbedModels {
			logger.Info("Start", "action", "proxy: adding embed model", "model", model.FQN())
		}
	}

	if cfg.Google.Credentials != "" {

		var err error
		client, err := vertexai.New(vertexai.GoogleConfig{
			Project:    cfg.Google.Project,
			Region:     cfg.Google.Region,
			Credential: cfg.Google.Credentials,
		})
		if err != nil {
			return nil, err
		}

		proxy.RegisterGen(client, slices.Collect(maps.Values(vertexai.GenModels))...)
		proxy.RegisterEmbeder(client, slices.Collect(maps.Values(vertexai.EmbedModels))...)

		for _, model := range vertexai.GenModels {
			logger.Info("Start", "action", "proxy: adding gen model", "model", model.FQN())
		}
		for _, model := range vertexai.EmbedModels {
			logger.Info("Start", "action", "proxy: adding embed model", "model", model.FQN())
		}
	}

	if cfg.VoyageAiKey != "" {
		client := voyageai.New(cfg.VoyageAiKey)
		proxy.RegisterEmbeder(client, slices.Collect(maps.Values(voyageai.EmbedModels))...)

		for _, model := range voyageai.EmbedModels {
			logger.Info("Start", "action", "proxy: adding embed model", "model", model.FQN())
		}

	}

	return proxy, nil
}
