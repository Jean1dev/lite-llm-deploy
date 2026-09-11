package httpapi

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/catalog"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/config"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/memory"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/provider"
	"github.com/Jean1dev/lite-llm-deploy/litellm-diet/internal/storage"
)

const shutdownTimeout = 15 * time.Second

type Dependencies struct {
	Port              int
	MasterKey         string
	Providers         map[provider.ID]config.Provider
	Catalog           catalog.Catalog
	Map               *memory.Map
	Aggregator        *memory.Aggregator
	Repo              *storage.Repository
	Client            *http.Client
	Ready             func(context.Context) error
	Now               func() time.Time
	Log               *slog.Logger
	SourceDatabaseURL string
	Importer          keyImporter
}

type Server struct {
	http              *http.Server
	log               *slog.Logger
	providers         map[provider.ID]config.Provider
	catalog           catalog.Catalog
	keys              *memory.Map
	aggregator        *memory.Aggregator
	repo              *storage.Repository
	client            *http.Client
	master            string
	ready             func(context.Context) error
	now               func() time.Time
	modelList         []byte
	modelInfo         []byte
	sourceDatabaseURL string
	importer          keyImporter
	migrateMu         sync.Mutex
}

func NewServer(dep Dependencies) (*Server, error) {
	if dep.Client == nil {
		dep.Client = &http.Client{Timeout: 10 * time.Minute}
	}
	if dep.Now == nil {
		dep.Now = time.Now
	}
	s := &Server{
		log:               dep.Log,
		providers:         dep.Providers,
		catalog:           dep.Catalog,
		keys:              dep.Map,
		aggregator:        dep.Aggregator,
		repo:              dep.Repo,
		client:            dep.Client,
		master:            dep.MasterKey,
		ready:             dep.Ready,
		now:               dep.Now,
		sourceDatabaseURL: dep.SourceDatabaseURL,
		importer:          dep.Importer,
	}
	if err := s.prepareCatalog(); err != nil {
		return nil, fmt.Errorf("prepare catalog: %w", err)
	}

	mux := http.NewServeMux()
	s.register(mux)
	s.http = &http.Server{
		Addr:              ":" + strconv.Itoa(dep.Port),
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      0,
		IdleTimeout:       90 * time.Second,
		MaxHeaderBytes:    1 << 16,
	}
	return s, nil
}

func (s *Server) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /health/live", s.liveness)
	mux.HandleFunc("GET /health/ready", s.readiness)

	mux.HandleFunc("POST /chat/completions", s.chatCompletions)
	mux.HandleFunc("POST /v1/chat/completions", s.chatCompletions)

	mux.HandleFunc("GET /models", s.listModels)
	mux.HandleFunc("GET /v1/models", s.listModels)
	mux.HandleFunc("GET /model/info", s.modelInfoHandler)
	mux.HandleFunc("GET /v1/model/info", s.modelInfoHandler)

	mux.HandleFunc("POST /key/generate", s.generateKey)
	mux.HandleFunc("POST /key/update", s.updateKey)
	mux.HandleFunc("POST /key/delete", s.deleteKey)
	mux.HandleFunc("POST /key/block", s.blockKey)
	mux.HandleFunc("POST /key/unblock", s.unblockKey)
	mux.HandleFunc("GET /key/info", s.keyInfo)
	mux.HandleFunc("GET /key/list", s.listKeys)
	mux.HandleFunc("POST /admin/migrate-keys", s.migrateKeys)
}

func (s *Server) Handler() http.Handler {
	return s.http.Handler
}

func (s *Server) Run(ctx context.Context) error {
	errs := make(chan error, 1)
	go func() { errs <- s.http.ListenAndServe() }()

	s.log.Info("http server listening", slog.String("addr", s.http.Addr))

	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("listen on %s: %w", s.http.Addr, err)
	case <-ctx.Done():
		if s.aggregator != nil {
			flush, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer cancel()
			if err := s.aggregator.Shutdown(flush); err != nil {
				s.log.Error("flush spend on shutdown", slog.String("err", err.Error()))
			}
		}
		stop, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()
		if err := s.http.Shutdown(stop); err != nil {
			return fmt.Errorf("shutdown http server: %w", err)
		}
		return nil
	}
}
