package profile

import (
	"context"
	"log/slog"
	"slices"
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

// scheduleAutoOff adds the nodes a manual apply booted to the pending
// auto-off and restarts its timer, so nothing booted by hand runs forever and
// nothing just booted is switched off early. Callers hold p.mu.
func (p *ProfileManager) scheduleAutoOff(booted []string) {
	if len(booted) == 0 {
		return
	}

	nodes := []string{}
	if p.autoOff != nil {
		nodes = append(nodes, p.autoOff.Nodes...)
	}
	for _, n := range booted {
		if !slices.Contains(nodes, n) {
			nodes = append(nodes, n)
		}
	}

	if p.timer != nil {
		p.timer.Stop()
	}
	// Each schedule gets a new generation, so a timer that already fired
	// while being replaced can't run the newer auto-off early.
	p.autoOffGen++
	gen := p.autoOffGen
	p.autoOff = &AutoOff{At: time.Now().Add(autoOffAfter), Nodes: nodes}
	p.timer = time.AfterFunc(autoOffAfter, func() { p.runAutoOff(gen) })

	slog.Info("profile auto-off scheduled", "at", p.autoOff.At, "nodes", nodes)
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
