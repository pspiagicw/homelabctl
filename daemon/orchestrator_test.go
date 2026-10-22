package daemon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/backend"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
)

// setupOutage creates an orchestrator during a confirmed outage with one OFF node.
func setupOutage(unmanaged bool) *Orchestrator {
	n := node.NewNode("desktop", config.NodeConfig{Address: "192.168.10.40", Unmanaged: unmanaged}, config.SSHConfig{})
	n.State = node.OFF
	n.PingFunc = func(string) bool { return false }
	n.WakeFunc = func(string) error { return nil }
	// Never fires, so the boot timer goroutine doesn't change state mid-test.
	n.AfterFunc = func(time.Duration) <-chan time.Time { return make(chan time.Time) }

	return &Orchestrator{
		Registry: &node.Registry{Nodes: map[string]*node.Node{"desktop": n}},
		Sentinel: &sentinel.Sentinel{State: sentinel.OutageConfirmed},
	}
}

// TestBootUnmanagedDuringOutage verifies unmanaged nodes can boot while the sentinel is down
func TestBootUnmanagedDuringOutage(t *testing.T) {
	o := setupOutage(true)

	_, err := o.Boot(context.Background(), "desktop")

	if err != nil {
		t.Errorf("Boot() error = %v, want nil", err)
	}
	if got := o.Registry.Nodes["desktop"].State; got != node.BOOTING {
		t.Errorf("node.State = %s, want %s", got, node.BOOTING)
	}
}

// TestBootManagedDuringOutageBlocked verifies managed nodes still can't boot during an outage
func TestBootManagedDuringOutageBlocked(t *testing.T) {
	o := setupOutage(false)

	_, err := o.Boot(context.Background(), "desktop")

	if !errors.Is(err, apierr.ErrConflict) {
		t.Errorf("Boot() error = %v, want ErrConflict", err)
	}
	if got := o.Registry.Nodes["desktop"].State; got != node.OFF {
		t.Errorf("node.State = %s, want %s", got, node.OFF)
	}
}

// The orchestrator must satisfy the interfaces its components call back into.
var (
	_ backend.API   = (*Orchestrator)(nil)
	_ backend.Power = (*Orchestrator)(nil)
)
