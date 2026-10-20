package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/pspiagicw/homelabctl/api"
	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/service"
	"github.com/pspiagicw/homelabctl/web"
)

// nodeRefreshInterval is how often every node is re-pinged in the background,
// so state set outside the daemon (manual power on/off) doesn't stay stale.
const nodeRefreshInterval = 15 * time.Minute

type Orchestrator struct {
	Registry *node.Registry
	Sentinel *sentinel.Sentinel
	Service  *service.Service
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
	o := &Orchestrator{}
	o.Registry = node.NewRegistry(cfg)
	o.Sentinel = sentinel.NewSentinel(cfg)
	o.Service = service.NewService(cfg)
	// Server also owns orchestrator!
	o.Server = api.NewServer(cfg, o)
	o.Server.UI = web.New(o).Handler()

	o.Sentinel.SetFunc(
		func(ctx context.Context) {
			o.ShutdownAll(ctx)
		},
		func(context.Context) {
			slog.Info("Restore function triggered!")
		},
	)

	return o
}

func (o *Orchestrator) Services() *service.ServiceList {
	return o.Service.GetServices()
}

func (o *Orchestrator) ListServices(ctx context.Context) (any, error) {
	list := o.Service.GetServices()

	return list, nil
}

func (o *Orchestrator) GetNode(ctx context.Context, name string) (any, error) {
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
	}, err
}

func (o *Orchestrator) NodeStates() *node.RegistryStatus {
	return o.Registry.Status()
}

func (o *Orchestrator) SentinelState() *sentinel.SentinelStatus {
	return o.Sentinel.Status()
}

func (o *Orchestrator) ListNodes(ctx context.Context) (any, error) {
	r := o.Registry.Status()

	return r, nil
}

// RefreshNodes re-pings every node now and returns the updated status.
func (o *Orchestrator) RefreshNodes(ctx context.Context) (any, error) {
	o.Registry.RefreshAll()

	return o.Registry.Status(), nil
}
func (o *Orchestrator) Status(ctx context.Context) (any, error) {
	s := o.Sentinel.Status()
	r := o.Registry.Status()
	d := &DaemonStatus{
		Sentinel: s,
		Registry: r,
	}
	return d, nil
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

func (o *Orchestrator) Shutdown(ctx context.Context, name string) (any, error) {
	err := o.Registry.Shutdown(ctx, name)

	message := ""
	if err != nil {
		message = err.Error()
	}

	return Response{
		err == nil,
		message,
	}, err

}

// TODO: Check if we need to send only true/false or node info too!
func (o *Orchestrator) Boot(ctx context.Context, name string) (any, error) {
	n, ok := o.Registry.Nodes[name]
	if !ok {
		err := fmt.Errorf("boot requested on invalid node; node: %s; %w", name, apierr.ErrNotFound)
		return Response{
			false,
			err.Error(),
		}, err
	}

	// Unmanaged nodes are outside the sentinel's control, so an outage doesn't block them.
	if !n.Unmanaged() && o.Sentinel.State != sentinel.Normal {
		return Response{
			false,
			"sentinel currently offline; can't boot any machines.",
		}, fmt.Errorf("sentinel currently offline; can't boot any machines; %w", apierr.ErrConflict)
	}

	err := o.Registry.Boot(ctx, name)

	message := ""
	if err != nil {
		message = err.Error()
	}

	return Response{
		err == nil,
		message,
	}, err
}

func (o *Orchestrator) Start(ctx context.Context) {
	slog.Info("orchestrator started!")
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		o.Server.Run(ctx)
		wg.Done()
	}()
	go func() {
		o.Sentinel.Run(ctx)
		wg.Done()
	}()
	go func() {
		o.Registry.Watch(ctx, nodeRefreshInterval)
		wg.Done()
	}()

	wg.Wait()
}
