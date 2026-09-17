package daemon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
)

type Orchestrator struct {
	Registry *node.Registry
	Sentinel *sentinel.Sentinel
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	o := &Orchestrator{
		Registry: node.NewRegistry(cfg),
		Sentinel: sentinel.NewSentinel(cfg),
	}

	return o
}

// TODO: Implement this kkkjj
func (o *Orchestrator) Init() {
	o.Registry.Init()
	o.Sentinel.Init()
	slog.Info("initialization completed!")
}

func (o *Orchestrator) PowerOff(ctx context.Context) {
	slog.Info("starting shutdown sequence")
	for name, node := range o.Registry.Nodes {
		slog.Info("shutdown requested", "node", name)
		node.Shutdown(ctx)
	}
}

func (o *Orchestrator) Start(ctx context.Context) {
	slog.Info("orchestrator started!")
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		o.Sentinel.Run(ctx, func(c context.Context) {
			o.PowerOff(ctx)
		},
			func(c context.Context) {
				slog.Info("Restore function triggered!")
			})
		wg.Done()
	}()

	wg.Wait()
}
