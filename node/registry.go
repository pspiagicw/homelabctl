package node

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
)

// nodeRefreshInterval is how often every node is re-pinged in the background,
// so state set outside the daemon (manual power on/off) doesn't stay stale.
var nodeRefreshInterval = 15 * time.Minute

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

// RefreshAll re-pings every node in parallel. Nodes mid-boot or mid-shutdown
// are left to their timers (see Node.refresh).
func (r *Registry) RefreshAll() {
	var wg sync.WaitGroup
	wg.Add(len(r.Nodes))
	for _, node := range r.Nodes {
		go func() {
			node.refresh()
			wg.Done()
		}()
	}
	wg.Wait()
	slog.Info("nodes refreshed", "count", len(r.Nodes))
}

// Watch calls RefreshAll every interval until ctx is cancelled.
func (r *Registry) Watch(ctx context.Context) {
	ticker := time.NewTicker(nodeRefreshInterval)
	defer ticker.Stop()
	slog.Info("node refresher started", "interval", nodeRefreshInterval)

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, stopping node refresher.")
			return
		case <-ticker.C:
			r.RefreshAll()
		}
	}
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
func (r *Registry) NodeStatus(ctx context.Context, name string) (*DetailedStatus, error) {
	node, ok := r.Nodes[name]
	if !ok {
		slog.Error("info requested on invalid node", "node", name)
		return nil, fmt.Errorf("info requested on invalid node; node: %s; %w", name, apierr.ErrNotFound)
	}

	return node.GetDetailedStatus(), nil
}

// ShutdownAll shuts down every managed node (unmanaged ones are skipped) and
// returns each attempted node's error, nil for a shutdown that was sent.
func (r *Registry) ShutdownAll(ctx context.Context) map[string]error {
	slog.Info("starting shutdown sequence")
	results := make(map[string]error, len(r.Nodes))
	for name, node := range r.Nodes {
		if node.Unmanaged() {
			slog.Info("skipping unmanaged node", "node", name)
			continue
		}
		slog.Info("shutdown requested", "node", name)
		results[name] = node.Shutdown(ctx)
	}
	return results
}

func (r *Registry) Boot(ctx context.Context, name string) error {
	node, ok := r.Nodes[name]
	if !ok {
		slog.Error("boot requested on invalid node", "node", name)
		return fmt.Errorf("boot requested on invalid node; node: %s; %w", name, apierr.ErrNotFound)
	}

	return node.Boot(ctx)
}

func (r *Registry) Shutdown(ctx context.Context, name string) error {
	node, ok := r.Nodes[name]
	if !ok {
		slog.Error("shutdown requested on invalid node", "node", name)
		return fmt.Errorf("shutdown requested on invalid node; node: %s; %w", name, apierr.ErrNotFound)
	}

	return node.Shutdown(ctx)
}
