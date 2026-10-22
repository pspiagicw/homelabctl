package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
)

// fakeBackend records boot order and returns a preset error per node.
type fakeBackend struct {
	mu       sync.Mutex
	booted   []string
	shutdown []string
	errs     map[string]error
}

func (f *fakeBackend) Boot(_ context.Context, name string) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.booted = append(f.booted, name)
	return nil, f.errs[name]
}
func (f *fakeBackend) Shutdown(_ context.Context, name string) (any, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.shutdown = append(f.shutdown, name)
	return nil, nil
}
func (f *fakeBackend) shutdowns() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.shutdown...)
}
func (f *fakeBackend) Status(context.Context) (any, error)               { return nil, nil }
func (f *fakeBackend) ListNodes(context.Context) (any, error)            { return nil, nil }
func (f *fakeBackend) GetNode(context.Context, string) (any, error)      { return nil, nil }
func (f *fakeBackend) ListServices(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) RefreshNodes(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) ListProfiles(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) GetProfile(context.Context, string) (any, error)   { return nil, nil }
func (f *fakeBackend) GetSchedule(context.Context) (any, error)          { return nil, nil }
func (f *fakeBackend) ShutdownAllNodes(context.Context) (any, error)     { return nil, nil }
func (f *fakeBackend) ApplyProfile(context.Context, string) (any, error) { return nil, nil }

// setupManager creates a manager with "movie" (two nodes) and "off" (none).
func setupManager(errs map[string]error) (*ProfileManager, *fakeBackend) {
	b := &fakeBackend{errs: errs}
	cfg := &config.Config{Profiles: map[string]config.ProfileConfig{
		"movie": {Nodes: []string{"falcon-heavy", "falcon2"}},
		"off":   {Nodes: []string{}},
	}}
	return NewProfileManager(cfg, b), b
}

func TestApplyBootsEveryNodeInOrder(t *testing.T) {
	p, b := setupManager(nil)

	applied, err := p.ApplyProfile(context.Background(), "movie", "manual")

	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}
	if fmt.Sprint(b.booted) != "[falcon-heavy falcon2]" {
		t.Errorf("booted %v, want [falcon-heavy falcon2]", b.booted)
	}
	for _, n := range applied.Nodes {
		if n.Result != Booting {
			t.Errorf("node %s result = %s, want %s", n.Node, n.Result, Booting)
		}
	}
}

// TestApplyContinuesPastSkippedAndFailed checks a node that's already on or
// fails doesn't stop the rest of the profile.
func TestApplyContinuesPastSkippedAndFailed(t *testing.T) {
	p, b := setupManager(map[string]error{
		"falcon-heavy": fmt.Errorf("node online, no need of boot; %w", apierr.ErrConflict),
		"falcon2":      fmt.Errorf("failed to send magic packet; %w", apierr.ErrUpstream),
	})

	applied, err := p.ApplyProfile(context.Background(), "movie", "manual")

	if err != nil {
		t.Fatalf("ApplyProfile() error = %v", err)
	}
	if len(b.booted) != 2 {
		t.Fatalf("booted %v, want both nodes attempted", b.booted)
	}
	if got := applied.Nodes[0]; got.Result != Skipped || got.Message == "" {
		t.Errorf("falcon-heavy = %+v, want skipped with a message", got)
	}
	if got := applied.Nodes[1]; got.Result != Failed || got.Message == "" {
		t.Errorf("falcon2 = %+v, want failed with a message", got)
	}
}

func TestApplyUnknownProfile(t *testing.T) {
	p, b := setupManager(nil)

	_, err := p.ApplyProfile(context.Background(), "nosuch", "manual")

	if !errors.Is(err, apierr.ErrNotFound) {
		t.Errorf("ApplyProfile() error = %v, want ErrNotFound", err)
	}
	if len(b.booted) != 0 {
		t.Errorf("booted %v, want nothing", b.booted)
	}
}

func TestListProfilesSortedWithLastApplied(t *testing.T) {
	p, _ := setupManager(nil)

	if p.ListProfiles().LastApplied != nil {
		t.Fatalf("LastApplied set before any apply")
	}
	p.ApplyProfile(context.Background(), "movie", "manual")

	status := p.ListProfiles()
	if status.Profiles[0].Name != "movie" || status.Profiles[1].Name != "off" {
		t.Errorf("profiles %+v, want sorted by name", status.Profiles)
	}
	last := status.LastApplied
	if last == nil || last.Profile != "movie" || last.Trigger != "manual" || last.AppliedAt.IsZero() {
		t.Errorf("LastApplied = %+v, want movie/manual with a time", last)
	}
}

// TestListProfilesReturnsCopies checks callers can't modify the manager's state.
func TestListProfilesReturnsCopies(t *testing.T) {
	p, _ := setupManager(nil)

	p.ListProfiles().Profiles[0].Nodes[0] = "changed"

	got, _ := p.GetProfile("movie")
	if got.Nodes[0] != "falcon-heavy" {
		t.Errorf("profile nodes changed through a copy: %v", got.Nodes)
	}
}

// TestListDuringApply reads while an apply records; run with -race.
func TestListDuringApply(t *testing.T) {
	p, _ := setupManager(nil)

	done := make(chan struct{})
	go func() {
		p.ApplyProfile(context.Background(), "movie", "manual")
		close(done)
	}()
	for range 50 {
		if _, err := json.Marshal(p.ListProfiles()); err != nil {
			t.Fatalf("json.Marshal(ListProfiles()) error = %v", err)
		}
	}
	<-done
}

// shortAutoOff shortens the auto-off delay for one test.
func shortAutoOff(t *testing.T, d time.Duration) {
	saved := autoOffAfter
	autoOffAfter = d
	t.Cleanup(func() { autoOffAfter = saved })
}

// waitFor polls cond until it's true or a second passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met within 1s")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func TestManualApplyAutoOffShutsDownBootedNodes(t *testing.T) {
	shortAutoOff(t, 20*time.Millisecond)
	p, b := setupManager(map[string]error{
		// already on: must not be shut down by the auto-off
		"falcon-heavy": fmt.Errorf("node online; %w", apierr.ErrConflict),
	})

	p.ApplyProfile(context.Background(), "movie", Manual)

	pending := p.ListProfiles().AutoOff
	if pending == nil || fmt.Sprint(pending.Nodes) != "[falcon2]" {
		t.Fatalf("AutoOff = %+v, want falcon2 pending", pending)
	}
	waitFor(t, func() bool { return p.ListProfiles().LastAutoOff != nil })

	if got := b.shutdowns(); fmt.Sprint(got) != "[falcon2]" {
		t.Errorf("shut down %v, want only [falcon2]", got)
	}
	status := p.ListProfiles()
	if status.AutoOff != nil {
		t.Errorf("AutoOff still pending after it ran")
	}
	if r := status.LastAutoOff.Nodes; len(r) != 1 || r[0].Result != ShuttingDown {
		t.Errorf("LastAutoOff = %+v, want falcon2 shutting", r)
	}
}

func TestAutoOffShutsDownInReverseBootOrder(t *testing.T) {
	shortAutoOff(t, 20*time.Millisecond)
	p, b := setupManager(nil)

	p.ApplyProfile(context.Background(), "movie", Manual)
	waitFor(t, func() bool { return len(b.shutdowns()) == 2 })

	if got := b.shutdowns(); fmt.Sprint(got) != "[falcon2 falcon-heavy]" {
		t.Errorf("shut down %v, want reverse boot order", got)
	}
}

func TestScheduledApplyHasNoAutoOff(t *testing.T) {
	p, _ := setupManager(nil)

	p.ApplyProfile(context.Background(), "movie", "schedule")

	if p.ListProfiles().AutoOff != nil {
		t.Errorf("AutoOff scheduled for a non-manual apply")
	}
}

// setupSwitching creates a manager over three managed nodes and one unmanaged
// node, with "movie" (falcon-heavy, falcon2), "backup" (falcon1, falcon-heavy)
// and "off" (nothing).
func setupSwitching() (*ProfileManager, *fakeBackend) {
	b := &fakeBackend{}
	cfg := &config.Config{
		Nodes: map[string]config.NodeConfig{
			"falcon-heavy": {}, "falcon1": {}, "falcon2": {},
			"starship": {Unmanaged: true},
		},
		Profiles: map[string]config.ProfileConfig{
			"movie":  {Nodes: []string{"falcon-heavy", "falcon2"}},
			"backup": {Nodes: []string{"falcon1", "falcon-heavy"}},
			"off":    {Nodes: []string{}},
		},
	}
	return NewProfileManager(cfg, b), b
}

func TestProfileListsWhatItShutsDown(t *testing.T) {
	p, _ := setupSwitching()

	movie, _ := p.GetProfile("movie")
	off, _ := p.GetProfile("off")

	if fmt.Sprint(movie.ShutsDown) != "[falcon1]" {
		t.Errorf("movie shuts down %v, want [falcon1]", movie.ShutsDown)
	}
	// Unmanaged starship is never shut down, not even by "off".
	if fmt.Sprint(off.ShutsDown) != "[falcon-heavy falcon1 falcon2]" {
		t.Errorf("off shuts down %v, want every managed node", off.ShutsDown)
	}
}

func TestApplyShutsDownOtherManagedNodes(t *testing.T) {
	p, b := setupSwitching()

	applied, _ := p.ApplyProfile(context.Background(), "movie", "schedule")

	if got := b.shutdowns(); fmt.Sprint(got) != "[falcon1]" {
		t.Errorf("shut down %v, want [falcon1]", got)
	}
	last := applied.Nodes[len(applied.Nodes)-1]
	if last.Node != "falcon1" || last.Result != ShuttingDown {
		t.Errorf("last result = %+v, want falcon1 shutting", last)
	}
}

// TestSwitchShutsDownInReversePreviousOrder checks nodes from the previous
// profile go down in reverse of its boot order.
func TestSwitchShutsDownInReversePreviousOrder(t *testing.T) {
	p, b := setupSwitching()

	p.ApplyProfile(context.Background(), "backup", "schedule") // falcon1, falcon-heavy
	p.ApplyProfile(context.Background(), "off", "schedule")

	// "off" shuts down everything: backup's nodes reversed, then the rest.
	got := b.shutdowns()[1:] // the first shutdown came from applying backup (falcon2)
	if fmt.Sprint(got) != "[falcon-heavy falcon1 falcon2]" {
		t.Errorf("shut down %v, want [falcon-heavy falcon1 falcon2]", got)
	}
}

// TestApplyCancelsPendingAutoOff checks a later apply, of any kind, replaces
// an earlier manual apply's auto-off, so it can't shut down nodes the new
// profile needs.
func TestApplyCancelsPendingAutoOff(t *testing.T) {
	p, _ := setupSwitching()

	p.ApplyProfile(context.Background(), "movie", Manual)
	if p.ListProfiles().AutoOff == nil {
		t.Fatal("no auto-off after a manual apply")
	}
	p.ApplyProfile(context.Background(), "backup", "schedule")

	if pending := p.ListProfiles().AutoOff; pending != nil {
		t.Errorf("AutoOff = %+v, want cancelled by the scheduled apply", pending)
	}
}

func TestSecondManualApplyReplacesAutoOff(t *testing.T) {
	p, _ := setupSwitching()

	p.ApplyProfile(context.Background(), "movie", Manual)
	first := p.ListProfiles().AutoOff.At
	time.Sleep(5 * time.Millisecond)
	p.ApplyProfile(context.Background(), "backup", Manual)

	pending := p.ListProfiles().AutoOff
	if fmt.Sprint(pending.Nodes) != "[falcon1 falcon-heavy]" {
		t.Errorf("pending nodes %v, want only backup's booted nodes", pending.Nodes)
	}
	if !pending.At.After(first) {
		t.Errorf("deadline not reset: %v, first %v", pending.At, first)
	}
}
