package node

import (
	"fmt"

	"github.com/pspiagicw/homelabctl/config"
)

type Registry struct {
	Nodes map[string]*Node
}

func NewRegistry(cfg *config.Config) (*Registry, error) {
	r := &Registry{
		Nodes: make(map[string]*Node, 3),
	}

	for name, node := range cfg.Nodes {
		n := Node{
			Name: name,
			cfg:  node,
			SSH:  cfg.SSH,
		}
		r.Nodes[name] = &n
	}

	return r, nil
}

func (r *Registry) Get(name string) (*Node, error) {
	node, ok := r.Nodes[name]
	if !ok {
		return nil, fmt.Errorf("error no node %s found in registry", name)
	}

	return node, nil
}
