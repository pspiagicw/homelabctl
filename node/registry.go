package node

import (
	"log/slog"

	"github.com/pspiagicw/homelabctl/config"
)

type Registry struct {
	Nodes map[string]*Node
}

func NewRegistry(cfg *config.Config) *Registry {
	r := &Registry{
		Nodes: make(map[string]*Node, 3),
	}

	for name, node := range cfg.Nodes {

		n := NewNode(name, node, cfg.SSH)
		r.Nodes[name] = n
	}

	return r
}

func (r *Registry) Init() {
	for _, node := range r.Nodes {
		node.Init()
	}
	slog.Info("registry initialized!")
}
