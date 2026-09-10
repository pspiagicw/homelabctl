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
}

func (o *Orchestrator) PowerOff(ctx context.Context) {
	for name, node := range o.Registry.Nodes {
		slog.Info("Powering off", "node", name)
		err := node.Shutdown(ctx)
		if err != nil {
			slog.Error("Error shutting down system", "error", err)
		}
	}
}

func (o *Orchestrator) Start(ctx context.Context) {
	o.Init()

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
