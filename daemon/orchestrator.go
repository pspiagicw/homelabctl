package daemon

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"

	"github.com/pspiagicw/homelabctl/api"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
)

type Orchestrator struct {
	Registry *node.Registry
	Sentinel *sentinel.Sentinel
	Server   *api.Server
}

type DaemonStatus struct {
	Sentinel *sentinel.SentinelStatus `json:"sentinel"`
	Registry *node.RegistryStatus     `json:"registry"`
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	o := &Orchestrator{
		Registry: node.NewRegistry(cfg),
		Sentinel: sentinel.NewSentinel(cfg),
		Server:   api.NewServer(cfg),
	}

	o.Sentinel.SetFunc(
		func(ctx context.Context) {
			o.PowerOff(ctx)
		},
		func(context.Context) {
			slog.Info("Restore function triggered!")
		},
	)

	o.Server.SetFunc(
		func(context.Context) []byte {
			status := o.Status()
			content, err := json.Marshal(status)
			if err != nil {
				slog.Info("error encoding daemon-status to json", "error", err)
			}

			return content
		},
		func(ctx context.Context, name string) []byte {
			o.Boot(ctx, name)
			return []byte{}
		},
	)

	return o
}

func (o *Orchestrator) Status() *DaemonStatus {
	s := o.Sentinel.Status()
	r := o.Registry.Status()
	d := &DaemonStatus{
		Sentinel: s,
		Registry: r,
	}
	return d
}

// TODO: Implement this kkkjj
func (o *Orchestrator) Init() {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		o.Registry.Init()
		wg.Done()
	}()
	go func() {
		o.Sentinel.Init()
		wg.Done()
	}()
	go func() {
		o.Server.Init()
		wg.Done()
	}()
	wg.Wait()
	slog.Info("initialization completed!")
}

func (o *Orchestrator) PowerOff(ctx context.Context) {
	slog.Info("starting shutdown sequence")
	for name, node := range o.Registry.Nodes {
		slog.Info("shutdown requested", "node", name)
		node.Shutdown(ctx)
	}
}

func (o *Orchestrator) Boot(ctx context.Context, bootName string) error {

	// TODO: Move this to registry
	for name, node := range o.Registry.Nodes {
		if name == bootName {
			slog.Info("sending boot command", "node", name)
			node.Boot(ctx)
		}
	}

	return nil
}

func (o *Orchestrator) Start(ctx context.Context) {
	slog.Info("orchestrator started!")
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		o.Server.Run(ctx)
		wg.Done()
	}()
	go func() {
		o.Sentinel.Run(ctx)
		wg.Done()
	}()

	wg.Wait()
}
