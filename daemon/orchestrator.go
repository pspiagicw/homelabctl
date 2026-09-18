package daemon

import (
	"context"
	"log/slog"
	"sync"

	"github.com/pspiagicw/homelabctl/api"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/utils"
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

type Response struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	o := &Orchestrator{
		Registry: node.NewRegistry(cfg),
		Sentinel: sentinel.NewSentinel(cfg),
		Server:   api.NewServer(cfg),
	}

	o.Sentinel.SetFunc(
		func(ctx context.Context) {
			o.ShutdownAll(ctx)
		},
		func(context.Context) {
			slog.Info("Restore function triggered!")
		},
	)

	o.Server.SetFunc(
		func(ctx context.Context) []byte {
			return utils.ToJSON(o.Status(ctx))
		},
		func(ctx context.Context, name string) []byte {
			return utils.ToJSON(o.Boot(ctx, name))
		},
		func(ctx context.Context, name string) []byte {
			return utils.ToJSON(o.Shutdown(ctx, name))
		},
		func(ctx context.Context) []byte {
			return utils.ToJSON(o.RegistryStatus(ctx))
		},
		func(ctx context.Context, name string) []byte {
			return utils.ToJSON(o.NodeStatus(ctx, name))
		},
	)

	return o
}

func (o *Orchestrator) NodeStatus(ctx context.Context, name string) any {
	n, err := o.Registry.NodeStatus(ctx, name)

	message := ""
	if err != nil {
		message = err.Error()
	}
	return struct {
		Status  bool                 `json:"status"`
		Message string               `json:"message"`
		Node    *node.DetailedStatus `json:"node"`
	}{
		err == nil,
		message,
		n,
	}
}

func (o *Orchestrator) RegistryStatus(ctx context.Context) *node.RegistryStatus {
	r := o.Registry.Status()

	return r
}
func (o *Orchestrator) Status(ctx context.Context) *DaemonStatus {
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

func (o *Orchestrator) ShutdownAll(ctx context.Context) {
	o.Registry.ShutdownAll(ctx)
}

func (o *Orchestrator) Shutdown(ctx context.Context, name string) any {
	err := o.Registry.Shutdown(ctx, name)

	message := ""
	if err != nil {
		message = err.Error()
	}

	return Response{
		err == nil,
		message,
	}

}

// TODO: Check if we need to send only true/false or node info too!
func (o *Orchestrator) Boot(ctx context.Context, name string) any {
	if o.Sentinel.State != sentinel.Normal {
		return Response{
			false,
			"sentinel currently offline; can't boot any machines.",
		}
	}
	err := o.Registry.Boot(ctx, name)

	message := ""
	if err != nil {
		message = err.Error()
	}

	return Response{
		err == nil,
		message,
	}
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
