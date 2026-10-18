package web

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/stretchr/testify/assert"
)

// fakeBackend records the node passed to Boot/Shutdown and returns bootErr from Boot.
type fakeBackend struct {
	nodes   map[string]*node.NodeStatus
	booted  string
	bootErr error
}

func (f *fakeBackend) NodeStates() *node.RegistryStatus {
	return &node.RegistryStatus{Nodes: f.nodes}
}
func (f *fakeBackend) SentinelState() *sentinel.SentinelStatus {
	return &sentinel.SentinelStatus{State: sentinel.Normal}
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
