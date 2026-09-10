package node

import (
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
}

// func (r *Registry) Get(name string) (*Node, error) {
// 	node, ok := r.Nodes[name]
// 	if !ok {
// 		return nil, fmt.Errorf("error no node %s found in registry", name)
// 	}
//
// 	return node, nil
// }
