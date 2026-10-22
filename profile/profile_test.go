package profile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
)

// fakeBackend records boot order and returns a preset error per node.
type fakeBackend struct {
	booted []string
	errs   map[string]error
}

func (f *fakeBackend) Boot(_ context.Context, name string) (any, error) {
	f.booted = append(f.booted, name)
	return nil, f.errs[name]
}
func (f *fakeBackend) Status(context.Context) (any, error)               { return nil, nil }
func (f *fakeBackend) ListNodes(context.Context) (any, error)            { return nil, nil }
func (f *fakeBackend) GetNode(context.Context, string) (any, error)      { return nil, nil }
func (f *fakeBackend) Shutdown(context.Context, string) (any, error)     { return nil, nil }
func (f *fakeBackend) ListServices(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) RefreshNodes(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) ListProfiles(context.Context) (any, error)         { return nil, nil }
func (f *fakeBackend) GetProfile(context.Context, string) (any, error)   { return nil, nil }
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
