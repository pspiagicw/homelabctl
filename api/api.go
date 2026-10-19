package api

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

type Backend interface {
	Status(ctx context.Context) (any, error)
	ListNodes(ctx context.Context) (any, error)
	GetNode(ctx context.Context, name string) (any, error)
	Boot(ctx context.Context, name string) (any, error)
	Shutdown(ctx context.Context, name string) (any, error)
	ListServices(ctx context.Context) (any, error)
	RefreshNodes(ctx context.Context) (any, error)
}

type Server struct {
	cfg *config.Config

	router  chi.Router
	backend Backend

	// UI is the web interface, mounted at /ui when set.
	UI http.Handler

	// StatusFunc   func(context.Context) []byte
	// BootFunc     func(context.Context, string) []byte
	// ShutdownFunc func(context.Context, string) []byte
	// NodesFunc    func(context.Context) []byte
	// NodeFunc     func(context.Context, string) []byte
}

func NewServer(config *config.Config, b Backend) *Server {
	s := &Server{
		cfg:     config,
		backend: b,
	}

	return s
}

// func (s *Server) SetFunc(statusFunc func(context.Context) []byte, bootFunc func(context.Context, string) []byte, shutdownFunc func(context.Context, string) []byte, nodesFunc func(context.Context) []byte, nodeFunc func(context.Context, string) []byte) {
// 	s.StatusFunc = statusFunc
// 	s.BootFunc = bootFunc
// 	s.ShutdownFunc = shutdownFunc
// 	s.NodesFunc = nodesFunc
// 	s.NodeFunc = nodeFunc
// }

func (s *Server) Init() {
	s.router = chi.NewRouter()
	s.router.Use(middleware.Logger)
	s.router.NotFound(func(w http.ResponseWriter, r *http.Request) {
		s.writeRouteError(w, http.StatusNotFound, "route not found")
	})
	s.router.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		s.writeRouteError(w, http.StatusMethodNotAllowed, "method not allowed")
	})
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hi!\n"))
	})
	s.router.Get("/status", s.handleStatus)

	s.router.Route("/nodes", func(r chi.Router) {
		r.Get("/", s.handleListNodes)
		r.Get("/{name}", s.handleGetNode)
		r.Post("/{name}/boot", s.handleBoot)
		r.Post("/{name}/shutdown", s.handleShutdown)
		r.Post("/refresh", s.handleRefreshNodes)
	})

	s.router.Get("/services", s.handleService)

	if s.UI != nil {
		s.router.Mount("/ui", s.UI)
		s.router.Get("/", func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, "/ui/", http.StatusFound)
		})
	}

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

func (s *Server) WriteResponse(response any, err error, w http.ResponseWriter) {

	w.Header().Set("Content-Type", "application/json")
	errCode := utils.ParseErrCode(err)
	w.WriteHeader(errCode)
	jsonResponse := utils.ToJSON(response)

	w.Write(jsonResponse)
}

func (s *Server) writeRouteError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(utils.ToJSON(struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
	}{false, message}))
}
func (s *Server) handleListNodes(w http.ResponseWriter, r *http.Request) {

	response, err := s.backend.ListNodes(r.Context())

	s.WriteResponse(response, err, w)
}

func (s *Server) handleGetNode(w http.ResponseWriter, r *http.Request) {

	name := chi.URLParam(r, "name")
	response, err := s.backend.GetNode(r.Context(), name)
	s.WriteResponse(response, err, w)

}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {

	response, err := s.backend.Status(r.Context())
	s.WriteResponse(response, err, w)

}

func (s *Server) handleBoot(w http.ResponseWriter, r *http.Request) {

	name := chi.URLParam(r, "name")
	response, err := s.backend.Boot(r.Context(), name)
	s.WriteResponse(response, err, w)
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {

	name := chi.URLParam(r, "name")

	response, err := s.backend.Shutdown(r.Context(), name)
	s.WriteResponse(response, err, w)

}

func (s *Server) handleService(w http.ResponseWriter, r *http.Request) {
	response, err := s.backend.ListServices(r.Context())
	s.WriteResponse(response, err, w)
}

func (s *Server) handleRefreshNodes(w http.ResponseWriter, r *http.Request) {
	response, err := s.backend.RefreshNodes(r.Context())
	s.WriteResponse(response, err, w)
}

// Handler returns the HTTP handler for this server.
// This allows tests to use httptest.NewServer(s.Handler()) or similar.
func (s *Server) Handler() http.Handler {
	return s.router
}
