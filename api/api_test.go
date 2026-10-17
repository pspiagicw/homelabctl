package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

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