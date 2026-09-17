package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pspiagicw/homelabctl/config"
)

type Server struct {
	cfg *config.Config

	router chi.Router

	StatusFunc func(context.Context) []byte
	BootFunc   func(context.Context, string) []byte
}

func NewServer(config *config.Config) *Server {
	s := &Server{
		cfg: config,
	}

	return s
}

func (s *Server) SetFunc(statusFunc func(context.Context) []byte, bootFunc func(context.Context, string) []byte) {
	s.StatusFunc = statusFunc
	s.BootFunc = bootFunc
}

func (s *Server) Init() {
	s.router = chi.NewRouter()
	s.router.Use(middleware.Logger)
	s.router.Get("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi!\n"))
	})
	s.router.Get("/status", s.handleStatus)

	s.router.Route("/nodes", func(r chi.Router) {
		// 	r.Get("/", s.handleListNodes)
		// 	r.Get("/name", s.handleGetNode)
		r.Post("/{name}/boot", s.handleBoot)
		// 	r.Post("/{name}/shutdown", s.handleShutdown)
	})

	slog.Info("server initialized!")
}

func (s *Server) Run(ctx context.Context) {
	// TODO: Implement context cancelling!
	slog.Info("api server started!")
	err := http.ListenAndServe(":3000", s.router)
	if err != nil {
		slog.Error("failed to listen on port 3000", "error", err)
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	response := s.StatusFunc(r.Context())

	w.Write(response)
}

func (s *Server) handleBoot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	name := chi.URLParam(r, "name")
	slog.Info("boot request", "node", name)

	response := s.BootFunc(r.Context(), name)

	w.Write(response)
}
