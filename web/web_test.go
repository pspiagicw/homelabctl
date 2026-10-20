package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/service"
	"github.com/pspiagicw/homelabctl/version"
	"github.com/stretchr/testify/assert"
)

// fakeBackend records the node passed to Boot/Shutdown and returns bootErr from Boot.
type fakeBackend struct {
	nodes     map[string]*node.NodeStatus
	services  []config.ServiceConfig
	sentinel  string // defaults to Normal when empty
	booted    string
	bootErr   error
	refreshed int // number of RefreshNodes calls
}

func (f *fakeBackend) NodeStates() *node.RegistryStatus {
	return &node.RegistryStatus{Nodes: f.nodes}
}
func (f *fakeBackend) SentinelState() *sentinel.SentinelStatus {
	if f.sentinel == "" {
		return &sentinel.SentinelStatus{State: sentinel.Normal}
	}
	return &sentinel.SentinelStatus{State: f.sentinel}
}
func (f *fakeBackend) RefreshNodes(_ context.Context) (any, error) {
	f.refreshed++
	return nil, nil
}
func (f *fakeBackend) Services() *service.ServiceList {
	return &service.ServiceList{Services: f.services}
}
func (f *fakeBackend) Boot(_ context.Context, name string) (any, error) {
	f.booted = name
	return nil, f.bootErr
}
func (f *fakeBackend) Shutdown(_ context.Context, _ string) (any, error) {
	return nil, nil
}

// setupUI creates a UI with two nodes in the given state.
func setupUI(state string) (*fakeBackend, http.Handler) {
	b := &fakeBackend{
		nodes: map[string]*node.NodeStatus{
			"falcon-heavy": {PowerState: state, Address: "192.168.10.45"},
			"falcon1":      {PowerState: state, Address: "192.168.10.44"},
		},
	}
	return b, New(b).Handler()
}

// disabledAttr matches a bare disabled attribute, not hx-disabled-elt.
var disabledAttr = regexp.MustCompile(`\sdisabled>`)

func serve(h http.Handler, method, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, nil))
	return rec
}

func TestIndexPage(t *testing.T) {
	_, h := setupUI(node.ON)

	rec := serve(h, http.MethodGet, "/")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "<html")
	assert.Contains(t, rec.Body.String(), "falcon-heavy")
	assert.Contains(t, rec.Body.String(), "falcon1")
	assert.Contains(t, rec.Body.String(), sentinel.Normal)
}

func TestNodesPartial(t *testing.T) {
	_, h := setupUI(node.ON)

	rec := serve(h, http.MethodGet, "/nodes")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), "<html")
	assert.Contains(t, rec.Body.String(), "falcon-heavy")
}

func TestBootAction(t *testing.T) {
	b, h := setupUI(node.OFF)

	rec := serve(h, http.MethodPost, "/nodes/falcon1/boot")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "falcon1", b.booted)
	assert.Contains(t, rec.Body.String(), "boot requested for falcon1")
}

func TestBootActionWithError(t *testing.T) {
	b, h := setupUI(node.ON)
	b.bootErr = fmt.Errorf("node online, no need of boot; %w", apierr.ErrConflict)

	rec := serve(h, http.MethodPost, "/nodes/falcon1/boot")

	assert.Equal(t, http.StatusConflict, rec.Code)
	assert.Contains(t, rec.Body.String(), "node online, no need of boot")
	assert.Contains(t, rec.Body.String(), "snack-error")
}

func TestBusyNodeButtonsDisabled(t *testing.T) {
	_, h := setupUI(node.BOOTING)

	rec := serve(h, http.MethodGet, "/nodes")

	assert.Regexp(t, disabledAttr, rec.Body.String())
}

// bootDisabled / shutdownDisabled match a disabled boot or shutdown button.
var (
	bootDisabled     = regexp.MustCompile(`class="btn btn-boot"[^>]*\sdisabled>`)
	shutdownDisabled = regexp.MustCompile(`class="btn btn-shutdown"[^>]*\sdisabled>`)
)

func TestOnNodeBootDisabled(t *testing.T) {
	_, h := setupUI(node.ON)

	rec := serve(h, http.MethodGet, "/nodes")

	assert.Regexp(t, bootDisabled, rec.Body.String())
	assert.NotRegexp(t, shutdownDisabled, rec.Body.String())
}

func TestOffNodeShutdownDisabled(t *testing.T) {
	_, h := setupUI(node.OFF)

	rec := serve(h, http.MethodGet, "/nodes")

	assert.Regexp(t, shutdownDisabled, rec.Body.String())
	assert.NotRegexp(t, bootDisabled, rec.Body.String())
}

func TestStaticAssets(t *testing.T) {
	_, h := setupUI(node.ON)

	for _, path := range []string{"/static/style.css", "/static/htmx.min.js", "/static/ui.js", "/static/favicon.svg"} {
		rec := serve(h, http.MethodGet, path)
		assert.Equal(t, http.StatusOK, rec.Code, path)
	}
}

// setupServicesUI adds services to a UI whose nodes are all in the given state.
func setupServicesUI(state string) http.Handler {
	b, _ := setupUI(state)
	b.services = []config.ServiceConfig{
		{Name: "Jellyfin", URL: "http://192.168.10.91:8096", Group: "Media", Node: "falcon-heavy"},
		{Name: "PiHole", URL: "http://192.168.10.92/admin", Group: "Infrastructure"},
		{Name: "Immich", URL: "http://192.168.10.99:2283", Group: "Media"},
		{Name: "Homer", URL: "http://192.168.10.85:8010"},
	}
	return New(b).Handler()
}

func TestServicesGroupedInConfigOrder(t *testing.T) {
	h := setupServicesUI(node.ON)

	body := serve(h, http.MethodGet, "/nodes").Body.String()

	// Groups appear in first-seen order, services keep list order inside a group,
	// and ungrouped services land in "other".
	assert.Regexp(t, `(?s)Media.*Jellyfin.*Immich.*Infrastructure.*PiHole.*other.*Homer`, body)
	assert.Contains(t, body, `href="http://192.168.10.91:8096"`)
	assert.Contains(t, body, "<code>192.168.10.91:8096</code>")
}

func TestServiceOnNodeOffIsDimmed(t *testing.T) {
	h := setupServicesUI(node.OFF)

	body := serve(h, http.MethodGet, "/nodes").Body.String()

	assert.Contains(t, body, "service-down")
	assert.Contains(t, body, "falcon-heavy is off")
}

func TestServiceOnNodeOnIsNotDimmed(t *testing.T) {
	h := setupServicesUI(node.ON)

	body := serve(h, http.MethodGet, "/nodes").Body.String()

	assert.NotContains(t, body, "service-down")
	assert.Contains(t, body, "falcon-heavy · on")
}

func TestNoServicesHidesSection(t *testing.T) {
	_, h := setupUI(node.ON)

	body := serve(h, http.MethodGet, "/nodes").Body.String()

	assert.NotContains(t, body, `class="services"`)
}

func TestUnmanagedNodeBootAllowedDuringOutage(t *testing.T) {
	b, _ := setupUI(node.OFF)
	b.sentinel = sentinel.OutageConfirmed
	b.nodes["falcon1"].Unmanaged = true
	h := New(b).Handler()

	body := serve(h, http.MethodGet, "/nodes").Body.String()

	// falcon1 (unmanaged) can boot; falcon-heavy (managed) stays blocked.
	assert.Contains(t, body, "chip-unmanaged")
	assert.Regexp(t, `(?s)hx-post="/ui/nodes/falcon-heavy/boot"[^>]*sentinel is not Normal" disabled>`, body)
	assert.NotRegexp(t, `(?s)hx-post="/ui/nodes/falcon1/boot"[^>]*disabled>`, body)
}

func TestRefreshAction(t *testing.T) {
	b, h := setupUI(node.ON)

	rec := serve(h, http.MethodPost, "/nodes/refresh")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, 1, b.refreshed)
	assert.Contains(t, rec.Body.String(), "refreshed 2 nodes")
}

func TestHeaderShowsVersion(t *testing.T) {
	_, h := setupUI(node.ON)

	body := serve(h, http.MethodGet, "/").Body.String()

	assert.Contains(t, body, `class="version" title="`+version.String()+`">`+version.Version+`</span>`)
}
