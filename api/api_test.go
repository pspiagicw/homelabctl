package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/stretchr/testify/assert"
)

// Simple fake backend for testing
type fakeBackend struct{}

func (f *fakeBackend) Status(_ context.Context) (any, error)    { return map[string]bool{"ok": true}, nil }
func (f *fakeBackend) ListNodes(_ context.Context) (any, error)  { return nil, nil }
func (f *fakeBackend) GetNode(_ context.Context, _ string) (any, error) {
	return nil, nil
}
func (f *fakeBackend) Boot(_ context.Context, _ string) (any, error) {
	return nil, nil
}
func (f *fakeBackend) Shutdown(_ context.Context, _ string) (any, error) {
	return nil, nil
}
func (f *fakeBackend) ListServices(_ context.Context) (any, error) {
	return map[string][]config.ServiceConfig{
		"services": {{Name: "Jellyfin", URL: "http://192.168.10.91:8096"}},
	}, nil
}
func (f *fakeBackend) RefreshNodes(_ context.Context) (any, error) {
	return map[string]bool{"refreshed": true}, nil
}

// Mock backend that returns errors for testing error cases
type errorBackend struct{}

func (e *errorBackend) Status(_ context.Context) (any, error)    { return nil, apierr.ErrUpstream }
func (e *errorBackend) ListNodes(_ context.Context) (any, error)  { return nil, apierr.ErrNotFound }
func (e *errorBackend) GetNode(_ context.Context, _ string) (any, error) {
	return nil, apierr.ErrConflict
}
func (e *errorBackend) Boot(_ context.Context, _ string) (any, error) {
	return nil, apierr.ErrUnavailable
}
func (e *errorBackend) Shutdown(_ context.Context, _ string) (any, error) {
	return nil, apierr.ErrUpstream
}
func (e *errorBackend) ListServices(_ context.Context) (any, error) {
	return nil, apierr.ErrUpstream
}
func (e *errorBackend) RefreshNodes(_ context.Context) (any, error) {
	return nil, apierr.ErrUpstream
}

func TestHealthEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/health", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status
	assert.Equal(t, http.StatusOK, rec.Code)
	
	// Check that we get the expected response body
	assert.Equal(t, "Hi!\n", rec.Body.String())
}

func TestStatusEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/status", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status (since our fake backend returns success)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestStatusEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/status", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get an upstream error status (502)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestListNodesEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/nodes", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status (since our fake backend returns success)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListNodesEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/nodes", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a not found status (404)
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestGetNodeEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/nodes/test-node", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status (since our fake backend returns success)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetNodeEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/nodes/test-node", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a conflict status (409)
	assert.Equal(t, http.StatusConflict, rec.Code)
}

func TestBootEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/test-node/boot", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status (since our fake backend returns success)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBootEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/test-node/boot", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get an unavailable status (503)
	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
}

func TestShutdownEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/test-node/shutdown", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status (since our fake backend returns success)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestShutdownEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/test-node/shutdown", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get an upstream error status (502)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestServicesEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}

	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/services", nil)
	rec := httptest.NewRecorder()

	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status and the service list as JSON
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.Contains(t, rec.Body.String(), `"name":"Jellyfin"`)
	assert.Contains(t, rec.Body.String(), `"url":"http://192.168.10.91:8096"`)
}

func TestServicesEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}

	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("GET", "/services", nil)
	rec := httptest.NewRecorder()

	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get an upstream error status (502)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestRefreshNodesEndpoint(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}

	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/refresh", nil)
	rec := httptest.NewRecorder()

	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 200 status and the backend's response
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), `"refreshed":true`)
}

func TestRefreshNodesEndpointWithError(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}

	// Create server with error backend
	server := NewServer(cfg, &errorBackend{})
	server.Init()

	// Create test request
	req := httptest.NewRequest("POST", "/nodes/refresh", nil)
	rec := httptest.NewRecorder()

	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get an upstream error status (502)
	assert.Equal(t, http.StatusBadGateway, rec.Code)
}

func TestRouteNotFound(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request to non-existent route
	req := httptest.NewRequest("GET", "/nonexistent", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 404 status 
	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestMethodNotAllowed(t *testing.T) {
	// Create a simple config (we don't need real values for this test)
	cfg := &config.Config{}
	
	// Create server with fake backend
	server := NewServer(cfg, &fakeBackend{})
	server.Init()

	// Create test request using POST on GET-only endpoint
	req := httptest.NewRequest("POST", "/health", nil)
	rec := httptest.NewRecorder()
	
	// Test the handler directly
	server.Handler().ServeHTTP(rec, req)

	// Check that we get a 405 status 
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
}
