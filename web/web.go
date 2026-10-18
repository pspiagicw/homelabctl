package web

import (
	"bytes"
	"context"
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"sort"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pspiagicw/homelabctl/logging"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/utils"
)

//go:embed templates static
var assets embed.FS

// Backend is what the UI needs from the orchestrator. Boot/Shutdown are the
// same calls the JSON API uses, so the UI follows the same rules.
type Backend interface {
	NodeStates() *node.RegistryStatus
	SentinelState() *sentinel.SentinelStatus
	Boot(ctx context.Context, name string) (any, error)
	Shutdown(ctx context.Context, name string) (any, error)
}

type UI struct {
	backend Backend
	tmpl    *template.Template
	static  fs.FS
	Log     *slog.Logger
}

type nodeRow struct {
	Name string
	*node.NodeStatus
}

type view struct {
	Sentinel *sentinel.SentinelStatus
	Nodes    []nodeRow
	Message  string
	Failed   bool
}

var funcs = template.FuncMap{
	// busy nodes are mid-transition; their timers resolve them, so actions are disabled.
	"busy": func(state string) bool {
		return state == node.BOOTING || state == node.SHUTTING
	},
	"seen": func(t time.Time) string {
		if t.IsZero() {
			return "never"
		}
		return t.Format("Jan 2, 15:04:05")
	},
}

func New(b Backend) *UI {
	static, _ := fs.Sub(assets, "static")
	return &UI{
		backend: b,
		tmpl:    template.Must(template.New("").Funcs(funcs).ParseFS(assets, "templates/*.html")),
		static:  static,
		Log:     logging.WithComponent("WEB"),
	}
}

func (u *UI) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/", u.handleIndex)
	r.Get("/nodes", u.handleNodes)
	r.Post("/nodes/{name}/boot", u.handleAction("boot", u.backend.Boot))
	r.Post("/nodes/{name}/shutdown", u.handleAction("shutdown", u.backend.Shutdown))
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, u.static, chi.URLParam(r, "*"))
	})
	return r
}

func (u *UI) view() view {
	status := u.backend.NodeStates()
	v := view{Sentinel: u.backend.SentinelState()}

	for name, n := range status.Nodes {
		v.Nodes = append(v.Nodes, nodeRow{name, n})
	}
	sort.Slice(v.Nodes, func(i, j int) bool { return v.Nodes[i].Name < v.Nodes[j].Name })

	return v
}

func (u *UI) handleIndex(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "index", u.view())
}

func (u *UI) handleNodes(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "panel", u.view())
}

func (u *UI) handleAction(verb string, do func(context.Context, string) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		name := chi.URLParam(r, "name")
		u.Log.Info("ui action requested", "action", verb, "node", name)

		_, err := do(r.Context(), name)

		v := u.view()
		if err != nil {
			v.Message = err.Error()
			v.Failed = true
		} else {
			v.Message = fmt.Sprintf("%s requested for %s", verb, name)
		}

		u.render(w, utils.ParseErrCode(err), "panel", v)
	}
}

func (u *UI) render(w http.ResponseWriter, code int, name string, v view) {
	var buf bytes.Buffer
	if err := u.tmpl.ExecuteTemplate(&buf, name, v); err != nil {
		u.Log.Error("failed to render template", "template", name, "error", err)
		http.Error(w, "failed to render page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	buf.WriteTo(w)
}
