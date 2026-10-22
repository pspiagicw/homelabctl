package profile

import (
	"context"
	"log/slog"
	"time"
)

// autoOffAfter is how long after a manual apply its booted nodes are shut
// down again. A var so tests can shorten it.
var autoOffAfter = time.Hour

// AutoOff is a pending shutdown of nodes booted by manual profile applies.
type AutoOff struct {
	At    time.Time `json:"at"`
	Nodes []string  `json:"nodes"` // in boot order; shut down in reverse
}

// AutoOffRun records what the last auto-off did.
type AutoOffRun struct {
	RanAt time.Time    `json:"ran_at"`
	Nodes []NodeResult `json:"nodes"`
}

// scheduleAutoOff shuts the nodes a manual apply booted down again after
// autoOffAfter. Callers hold p.mu and have cancelled any earlier auto-off.
func (p *ProfileManager) scheduleAutoOff(booted []string) {
	if len(booted) == 0 {
		return
	}

	// Each schedule gets a new generation, so a timer that already fired
	// while being cancelled can't run a newer auto-off early.
	p.autoOffGen++
	gen := p.autoOffGen
	p.autoOff = &AutoOff{At: time.Now().Add(autoOffAfter), Nodes: booted}
	p.timer = time.AfterFunc(autoOffAfter, func() { p.runAutoOff(gen) })

	slog.Info("profile auto-off scheduled", "at", p.autoOff.At, "nodes", booted)
}

// cancelAutoOff drops any pending auto-off. Callers hold p.mu.
func (p *ProfileManager) cancelAutoOff() {
	if p.timer != nil {
		p.timer.Stop()
	}
	if p.autoOff != nil {
		slog.Info("profile auto-off cancelled", "nodes", p.autoOff.Nodes)
	}
	p.autoOff, p.timer = nil, nil
	p.autoOffGen++
}

// runAutoOff shuts down the pending nodes in reverse boot order, unless the
// auto-off was rescheduled after this timer was set.
func (p *ProfileManager) runAutoOff(gen int) {
	p.mu.Lock()
	if gen != p.autoOffGen {
		p.mu.Unlock()
		return
	}
	pending := p.autoOff
	p.autoOff = nil
	p.timer = nil
	p.mu.Unlock()

	if pending == nil {
		return
	}

	run := &AutoOffRun{Nodes: make([]NodeResult, 0, len(pending.Nodes))}
	for i := len(pending.Nodes) - 1; i >= 0; i-- {
		node := pending.Nodes[i]
		_, err := p.b.Shutdown(context.Background(), node)
		run.Nodes = append(run.Nodes, ShutdownResult(node, err))
	}
	run.RanAt = time.Now()

	slog.Info("profile auto-off ran", "nodes", pending.Nodes)

	p.mu.Lock()
	p.lastAutoOff = run
	p.mu.Unlock()
}
