// Package catalog holds the services listed in the config (the links shown on
// the dashboard) and, later, their health checks.
package catalog

import (
	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

type Catalog struct {
	PingFunc  agents.Pinger
	AfterFunc agents.After

	cfg config.ServicesConfig
}

func NewCatalog(cfg *config.Config) *Catalog {
	return &Catalog{
		PingFunc:  utils.Ping,
		AfterFunc: utils.After,

		cfg: cfg.Services,
	}
}

// List is the catalog's services in config order, as served by GET /services.
type List struct {
	Services []config.ServiceConfig `json:"services"`
}

func (c *Catalog) List() *List {
	list := &List{
		Services: c.cfg,
	}

	return list
}
