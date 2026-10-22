// Package catalog holds the services listed in the config (the links shown on
// the dashboard) and, later, their health checks.
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

type CatalogService struct {
	config.ServiceConfig

	LastPing bool      `json:"last_ping"`
	LastSeen time.Time `json:"last_seen"`

	// Functions can't be encoded to JSON; keep it out of GET /services.
	PingFunc agents.Pinger `json:"-"`
}

func (c *CatalogService) refresh() {
	status := c.PingFunc(c.URL)
	c.LastPing = status

	if status {
		c.LastSeen = time.Now()
	}
}

type Catalog struct {
	PingFunc  agents.Pinger
	AfterFunc agents.After

	// cfg config.ServicesConfig
	services []*CatalogService
}

func NewCatalog(cfg *config.Config) *Catalog {
	services := []*CatalogService{}

	for _, service := range cfg.Services {
		c := &CatalogService{
			service,
			false,
			time.Time{},
			utils.Ping,
		}
		services = append(services, c)
	}

	return &Catalog{
		PingFunc:  utils.Ping,
		AfterFunc: utils.After,

		services: services,
	}
}

// List is the catalog's services in config order, as served by GET /services.
type List struct {
	Services []*CatalogService `json:"services"`
}

func (c *Catalog) List() *List {
	list := &List{
		Services: c.services,
	}

	return list
}

func (c *Catalog) Watch(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, shutting down catalog watcher.")
			return
		case <-ticker.C:
			length := len(c.services)
			var wg sync.WaitGroup
			wg.Add(length)
			for _, service := range c.services {
				go func() {
					service.refresh()
					wg.Done()
				}()
			}
			wg.Wait()
		}
	}

}
