// Package catalog holds the services listed in the config (the links shown on
// the dashboard) and checks every minute whether each one answers.
package catalog

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

// Status is a service and the result of its latest check. It's a plain value,
// so the API and UI get copies they can read without locking.
type Status struct {
	config.ServiceConfig

	LastPing bool      `json:"last_ping"`
	LastSeen time.Time `json:"last_seen"`
}

// CatalogService is one service being checked. Its status is only touched
// under mu, because the watcher writes it while the API and UI read it.
type CatalogService struct {
	mu      sync.Mutex
	status  Status
	checked bool // whether a check has completed yet

	CheckFunc agents.Pinger
}

// refresh runs the check without holding the lock (it can take up to the
// check timeout), then records the result.
func (s *CatalogService) refresh() {
	up := s.CheckFunc(s.status.URL)

	s.mu.Lock()
	first := !s.checked
	changed := s.status.LastPing != up
	s.checked = true
	s.status.LastPing = up
	if up {
		s.status.LastSeen = time.Now()
	}
	s.mu.Unlock()

	// Log the first result and every change; routine checks only at debug.
	if first || changed {
		slog.Info("service status", "service", s.status.Name, "url", s.status.URL, "up", up)
	} else {
		slog.Debug("service checked", "service", s.status.Name, "up", up)
	}
}

// snapshot returns a copy of the current status.
func (s *CatalogService) snapshot() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.status
}

type Catalog struct {
	services []*CatalogService
}

func NewCatalog(cfg *config.Config) *Catalog {
	services := []*CatalogService{}

	for _, service := range cfg.Services {
		c := &CatalogService{
			status:    Status{ServiceConfig: service},
			CheckFunc: utils.HTTPCheck,
		}
		services = append(services, c)
	}

	return &Catalog{
		services: services,
	}
}

// List is the catalog's services in config order, as served by GET /services.
type List struct {
	Services []Status `json:"services"`
}

// List returns a snapshot of every service's status.
func (c *Catalog) List() *List {
	list := &List{
		Services: make([]Status, 0, len(c.services)),
	}

	for _, service := range c.services {
		list.Services = append(list.Services, service.snapshot())
	}

	return list
}

func (c *Catalog) Init() {
	c.Refresh()
	slog.Info("catalog initialized!", "services", len(c.services))
}

// Refresh checks every service in parallel and waits for all of them.
func (c *Catalog) Refresh() {
	var wg sync.WaitGroup
	wg.Add(len(c.services))
	for _, service := range c.services {
		go func() {
			service.refresh()
			wg.Done()
		}()
	}
	wg.Wait()
}

func (c *Catalog) Watch(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, shutting down catalog watcher.")
			return
		case <-ticker.C:
			c.Refresh()
		}
	}
}
