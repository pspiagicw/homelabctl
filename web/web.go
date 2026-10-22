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
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/pspiagicw/homelabctl/catalog"
	"github.com/pspiagicw/homelabctl/logging"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/profile"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/utils"
	"github.com/pspiagicw/homelabctl/version"
)

//go:embed templates static
var assets embed.FS

// Backend is what the UI needs from the orchestrator. Boot/Shutdown are the
// same calls the JSON API uses, so the UI follows the same rules.
type Backend interface {
	NodeStates() *node.RegistryStatus
	SentinelState() *sentinel.SentinelStatus
	Services() *catalog.List
	ProfileStatus() *profile.ProfileStatus
	ApplyProfile(ctx context.Context, name string) (any, error)
	RefreshNodes(ctx context.Context) (any, error)
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

// serviceTile is a service plus what the dashboard shows about it.
type serviceTile struct {
	catalog.Status
	Host        string // host[:port] from URL, shown under the description
	NodeState   string // power state of Node; empty when Node is unset
	Health      string // up, down, asleep or unchecked
	HealthTitle string // hover text: when it was last checked
}

// health classifies a service from its last check and its node's power state.
// A service that doesn't answer while its node is off is asleep, not down.
func health(s catalog.Status, nodeState string) string {
	switch {
	case s.LastChecked.IsZero():
		return "unchecked"
	case s.LastPing:
		return "up"
	case s.Node != "" && nodeState != node.ON:
		return "asleep"
	default:
		return "down"
	}
}

// healthTitle is the hover text for a service's health indicator.
func healthTitle(s catalog.Status, h string) string {
	switch h {
	case "unchecked":
		return "not checked yet"
	case "asleep":
		return fmt.Sprintf("not answering: %s is off · checked %s", s.Node, s.LastChecked.Format("15:04:05"))
	case "down":
		return fmt.Sprintf("not answering · checked %s", s.LastChecked.Format("15:04:05"))
	default:
		return fmt.Sprintf("%s · checked %s", h, s.LastChecked.Format("15:04:05"))
	}
}

type serviceGroup struct {
	Name     string
	Services []serviceTile
}

type view struct {
	Version  string // e.g. v0.2.0, shown in the header
	Build    string // version, commit and build date, shown on hover
	Sentinel *sentinel.SentinelStatus
	Nodes    []nodeRow
	Services []serviceGroup
	Profiles *profile.ProfileStatus
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
	"join": func(items []string) string {
		return strings.Join(items, ", ")
	},
	// initial is the letter shown when a service icon is missing or fails to load.
	"initial": func(name string) string {
		for _, r := range name {
			return strings.ToUpper(string(r))
		}
		return "?"
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
	r.Post("/nodes/refresh", u.handleRefresh)
	r.Post("/profiles/{name}/apply", u.handleApplyProfile)
	r.Get("/static/*", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFileFS(w, r, u.static, chi.URLParam(r, "*"))
	})
	return r
}

func (u *UI) view() view {
	status := u.backend.NodeStates()
	v := view{
		Version:  version.Version,
		Build:    version.String(),
		Sentinel: u.backend.SentinelState(),
	}

	for name, n := range status.Nodes {
		v.Nodes = append(v.Nodes, nodeRow{name, n})
	}
	sort.Slice(v.Nodes, func(i, j int) bool { return v.Nodes[i].Name < v.Nodes[j].Name })

	states := make(map[string]string, len(v.Nodes))
	for _, n := range v.Nodes {
		states[n.Name] = n.PowerState
	}
	v.Profiles = u.backend.ProfileStatus()
	v.Services = groupServices(u.backend.Services().Services, states)

	return v
}

// groupServices keeps config order: groups in first-seen order, and services
// in list order within each group.
func groupServices(services []catalog.Status, states map[string]string) []serviceGroup {
	var groups []serviceGroup
	index := make(map[string]int)

	for _, s := range services {
		tile := serviceTile{Status: s, Host: s.URL, NodeState: states[s.Node]}
		tile.Health = health(s, tile.NodeState)
		tile.HealthTitle = healthTitle(s, tile.Health)
		if parsed, err := url.Parse(s.URL); err == nil && parsed.Host != "" {
			tile.Host = parsed.Host
		}

		i, ok := index[s.Group]
		if !ok {
			i = len(groups)
			index[s.Group] = i
			groups = append(groups, serviceGroup{Name: s.Group})
		}
		groups[i].Services = append(groups[i].Services, tile)
	}

	return groups
}

func (u *UI) handleIndex(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "index", u.view())
}

func (u *UI) handleNodes(w http.ResponseWriter, r *http.Request) {
	u.render(w, http.StatusOK, "panel", u.view())
}

func (u *UI) handleApplyProfile(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	u.Log.Info("ui profile apply requested", "profile", name)

	_, err := u.backend.ApplyProfile(r.Context(), name)

	v := u.view()
	switch {
	case err != nil:
		v.Message = err.Error()
		v.Failed = true
	case v.Profiles.LastApplied != nil:
		v.Message, v.Failed = appliedMessage(v.Profiles.LastApplied)
	}

	u.render(w, utils.ParseErrCode(err), "panel", v)
}

// appliedMessage summarises a profile apply for the event history, e.g.
// "applied movie: booting falcon-heavy, skipped falcon2". It counts as failed
// when any node failed to boot.
func appliedMessage(a *profile.Applied) (string, bool) {
	if len(a.Nodes) == 0 {
		return fmt.Sprintf("applied %s: nothing to boot", a.Profile), false
	}

	failed := false
	parts := make([]string, 0, len(a.Nodes))
	for _, n := range a.Nodes {
		parts = append(parts, n.Result+" "+n.Node)
		failed = failed || n.Result == profile.Failed
	}

	return fmt.Sprintf("applied %s: %s", a.Profile, strings.Join(parts, ", ")), failed
}

func (u *UI) handleRefresh(w http.ResponseWriter, r *http.Request) {
	u.Log.Info("ui refresh requested")

	_, err := u.backend.RefreshNodes(r.Context())

	v := u.view()
	if err != nil {
		v.Message = err.Error()
		v.Failed = true
	} else {
		v.Message = fmt.Sprintf("refreshed %d nodes", len(v.Nodes))
	}

	u.render(w, utils.ParseErrCode(err), "panel", v)
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
