// Package profile holds the named sets of nodes from the config. Applying a
// profile boots every node it lists; nodes outside the profile are left alone.
package profile

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/backend"
	"github.com/pspiagicw/homelabctl/config"
)

// Per-node outcomes of applying a profile.
const (
	Booting = "booting" // Wake-on-LAN sent
	Skipped = "skipped" // nothing to do, e.g. already on or mid-transition
	Failed  = "failed"  // boot attempted and failed
)

type Profile struct {
	Name  string   `json:"name"`
	Nodes []string `json:"nodes"` // boot order
}

type NodeResult struct {
	Node    string `json:"node"`
	Result  string `json:"result"`
	Message string `json:"message,omitempty"`
}

// Applied records one application of a profile.
type Applied struct {
	Profile   string       `json:"profile"`
	Trigger   string       `json:"trigger"` // "manual", later "schedule"
	AppliedAt time.Time    `json:"applied_at"`
	Nodes     []NodeResult `json:"nodes"`
}

type ProfileStatus struct {
	Profiles    []Profile `json:"profiles"`     // sorted by name
	LastApplied *Applied  `json:"last_applied"` // nil until the first apply
}

type ProfileManager struct {
	// applyMu serialises applies. mu guards last and is only held briefly,
	// so reads aren't blocked while an apply waits on node boots.
	applyMu sync.Mutex
	mu      sync.Mutex

	profiles map[string]Profile
	last     *Applied

	b backend.API
}

func NewProfileManager(cfg *config.Config, o backend.API) *ProfileManager {
	profiles := map[string]Profile{}

	for name, profile := range cfg.Profiles {
		profiles[name] = Profile{
			Name:  name,
			Nodes: profile.Nodes,
		}
	}
	return &ProfileManager{
		profiles: profiles,
		b:        o,
	}
}

// ListProfiles returns a copy of every profile, sorted by name, and the last
// applied result.
func (p *ProfileManager) ListProfiles() *ProfileStatus {
	status := &ProfileStatus{
		Profiles: make([]Profile, 0, len(p.profiles)),
	}

	for _, profile := range p.profiles {
		status.Profiles = append(status.Profiles, copyProfile(profile))
	}
	sort.Slice(status.Profiles, func(i, j int) bool {
		return status.Profiles[i].Name < status.Profiles[j].Name
	})

	p.mu.Lock()
	defer p.mu.Unlock()
	if p.last != nil {
		last := *p.last
		last.Nodes = append([]NodeResult(nil), p.last.Nodes...)
		status.LastApplied = &last
	}

	return status
}

func (p *ProfileManager) GetProfile(name string) (Profile, error) {
	profile, ok := p.profiles[name]
	if !ok {
		return Profile{}, fmt.Errorf("no such profile: %s; %w", name, apierr.ErrNotFound)
	}

	return copyProfile(profile), nil
}

// ApplyProfile boots every node in the profile, in order, and records the
// result. A node that can't be booted doesn't stop the others.
func (p *ProfileManager) ApplyProfile(ctx context.Context, name, trigger string) (*Applied, error) {
	profile, err := p.GetProfile(name)
	if err != nil {
		return nil, err
	}

	p.applyMu.Lock()
	defer p.applyMu.Unlock()

	applied := &Applied{
		Profile: name,
		Trigger: trigger,
		Nodes:   make([]NodeResult, 0, len(profile.Nodes)),
	}

	for _, node := range profile.Nodes {
		_, err := p.b.Boot(ctx, node)
		applied.Nodes = append(applied.Nodes, result(node, err))
	}
	applied.AppliedAt = time.Now()

	p.mu.Lock()
	p.last = applied
	p.mu.Unlock()

	return applied, nil
}

// result classifies a boot attempt. Conflicts (already on, mid-transition)
// mean there was nothing to do, not that the profile failed.
func result(node string, err error) NodeResult {
	switch {
	case err == nil:
		return NodeResult{Node: node, Result: Booting}
	case errors.Is(err, apierr.ErrConflict):
		return NodeResult{Node: node, Result: Skipped, Message: err.Error()}
	default:
		return NodeResult{Node: node, Result: Failed, Message: err.Error()}
	}
}

// copyProfile returns a profile with its own node slice, never nil, so an
// empty profile encodes as "nodes": [] rather than null.
func copyProfile(profile Profile) Profile {
	nodes := make([]string, len(profile.Nodes))
	copy(nodes, profile.Nodes)
	profile.Nodes = nodes
	return profile
}
