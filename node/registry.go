package node

import (
	"log/slog"
	"sync"

	"github.com/pspiagicw/homelabctl/config"
)

type Registry struct {
	Nodes map[string]*Node
}

type RegistryStatus struct {
	Nodes map[string]*NodeStatus `json:"nodes"`
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
	var wg sync.WaitGroup
	wg.Add(len(r.Nodes))
	for _, node := range r.Nodes {
		go func() {
			node.Init()
			wg.Done()
		}()
	}
	wg.Wait()
	slog.Info("registry initialized!")
}

func (r *Registry) Status() *RegistryStatus {
	status := &RegistryStatus{
		Nodes: map[string]*NodeStatus{},
	}

	for name, node := range r.Nodes {
		status.Nodes[name] = node.Status()
	}

	return status
}
