package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"

	"github.com/pspiagicw/homelabctl/api"
	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/catalog"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/logging"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/profile"
	"github.com/pspiagicw/homelabctl/schedule"
	"github.com/pspiagicw/homelabctl/sentinel"
	"github.com/pspiagicw/homelabctl/web"
)

type Orchestrator struct {
	Registry *node.Registry
	Sentinel *sentinel.Sentinel
	Catalog  *catalog.Catalog
	Server   *api.Server
	Profiles *profile.ProfileManager
	Schedule *schedule.Scheduler
}

func NewOrchestrator(cfg *config.Config) *Orchestrator {
	o := &Orchestrator{}
	o.Registry = node.NewRegistry(cfg)
	o.Sentinel = sentinel.NewSentinel(cfg, o)
	o.Catalog = catalog.NewCatalog(cfg)
	o.Server = api.NewServer(cfg, o)
	o.Server.UI = web.New(o).Handler()
	o.Profiles = profile.NewProfileManager(cfg, o)

	sched, err := schedule.NewScheduler(cfg, o)
	if err != nil {
		logging.Fatal("invalid schedule", "error", err)
	}
	o.Schedule = sched

	return o
}

func (o *Orchestrator) Init() {
	var wg sync.WaitGroup
	wg.Add(4)
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
	go func() {
		o.Catalog.Init()
		wg.Done()
	}()
	wg.Wait()
	slog.Info("initialization completed!")
}

func (o *Orchestrator) Start(ctx context.Context) {
	slog.Info("orchestrator started!")
	var wg sync.WaitGroup
	wg.Add(5)
	go func() {
		o.Server.Run(ctx)
		wg.Done()
	}()
	go func() {
		o.Sentinel.Watch(ctx)
		wg.Done()
	}()
	go func() {
		o.Registry.Watch(ctx)
		wg.Done()
	}()
	go func() {
		o.Catalog.Watch(ctx)
		wg.Done()
	}()
	go func() {
		o.Schedule.Watch(ctx)
		wg.Done()
	}()

	wg.Wait()
}

func (o *Orchestrator) Services() *catalog.List {
	return o.Catalog.List()
}

func (o *Orchestrator) ListServices(ctx context.Context) (any, error) {
	list := o.Catalog.List()

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

func (o *Orchestrator) Status(ctx context.Context) (any, error) {
	s := o.Sentinel.Status()
	r := o.Registry.Status()
	d := &DaemonStatus{
		Sentinel: s,
		Registry: r,
	}
	return d, nil
}

// RefreshNodes re-pings every node now and returns the updated status.
func (o *Orchestrator) RefreshNodes(ctx context.Context) (any, error) {
	o.Registry.RefreshAll()

	return o.Registry.Status(), nil
}

func (o *Orchestrator) ShutdownAll(ctx context.Context) {
	o.Registry.ShutdownAll(ctx)
}

// ShutdownAllNodes is the user-requested version of the outage shutdown: every
// managed node is shut down, unmanaged nodes are left alone. It returns each
// node's result; nodes that were already off show as skipped.
func (o *Orchestrator) ShutdownAllNodes(ctx context.Context) (any, error) {
	return o.ShutdownAllResults(ctx), nil
}

// ShutdownAllResults runs ShutdownAllNodes and returns typed results for the
// web UI, in node-name order.
func (o *Orchestrator) ShutdownAllResults(ctx context.Context) []profile.NodeResult {
	errs := o.Registry.ShutdownAll(ctx)

	names := make([]string, 0, len(errs))
	for name := range errs {
		names = append(names, name)
	}
	sort.Strings(names)

	results := make([]profile.NodeResult, 0, len(names))
	for _, name := range names {
		results = append(results, profile.ShutdownResult(name, errs[name]))
	}
	return results
}

// RestoreAll runs when mains power is back. Nodes aren't booted yet; that
// comes with the scheduler/profiles.
func (o *Orchestrator) RestoreAll(ctx context.Context) {
	slog.Info("Restore function triggered!")
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
	// Managed nodes can't boot while the sentinel isn't Normal: that would wake
	// them onto battery power. Unmanaged nodes are outside the sentinel's
	// control. Unknown nodes fall through to Registry.Boot, which returns 404.
	if n, ok := o.Registry.Nodes[name]; ok && !n.Unmanaged() && o.Sentinel.State != sentinel.Normal {
		err := fmt.Errorf("sentinel currently offline; can't boot any machines; %w", apierr.ErrConflict)
		return Response{false, err.Error()}, err
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

// ApplyProfile boots every node in the profile on request (UI/API); manual
// applies also schedule an auto-off.
func (o *Orchestrator) ApplyProfile(ctx context.Context, name string) (any, error) {
	applied, err := o.applyProfile(ctx, name, profile.Manual)
	if err != nil {
		return Response{false, err.Error()}, err
	}

	return applied, nil
}

// ApplyScheduledProfile is called by the scheduler; it never schedules an
// auto-off.
func (o *Orchestrator) ApplyScheduledProfile(ctx context.Context, name string) (*profile.Applied, error) {
	return o.applyProfile(ctx, name, schedule.Trigger)
}

// applyProfile refuses the whole profile while the sentinel isn't Normal:
// booting onto battery power is never wanted.
func (o *Orchestrator) applyProfile(ctx context.Context, name, trigger string) (*profile.Applied, error) {
	if o.Sentinel.State != sentinel.Normal {
		return nil, fmt.Errorf("sentinel not Normal; profile %s not applied; %w", name, apierr.ErrConflict)
	}

	return o.Profiles.ApplyProfile(ctx, name, trigger)
}

func (o *Orchestrator) GetProfile(ctx context.Context, name string) (any, error) {
	p, err := o.Profiles.GetProfile(name)
	if err != nil {
		return Response{false, err.Error()}, err
	}

	return p, nil
}

func (o *Orchestrator) ListProfiles(ctx context.Context) (any, error) {
	return o.Profiles.ListProfiles(), nil
}

// ProfileStatus is the typed profile overview for the web UI.
func (o *Orchestrator) ProfileStatus() *profile.ProfileStatus {
	return o.Profiles.ListProfiles()
}

func (o *Orchestrator) GetSchedule(ctx context.Context) (any, error) {
	return o.Schedule.Status(), nil
}

// ScheduleStatus is the typed schedule overview for the web UI.
func (o *Orchestrator) ScheduleStatus() *schedule.Status {
	return o.Schedule.Status()
}
