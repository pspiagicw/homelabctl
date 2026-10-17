package node

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

// falcon-heavy:
//   address: 192.168.1.12
//   mac: "b8:27:eb:00:00:02"
//   wait_after_boot: 180        # static delay before considered "ready"
//   services:
//     - jellyfin
//     - immich
//     - truenas

const (
	UNKNOWN     string = "unknown"
	OFF         string = "off"
	BOOTING     string = "booting"
	SHUTTING    string = "shutting"
	ON          string = "on"
	UNREACHABLE string = "unreachable"
)

// TODO: Implement services workflow
type Node struct {
	Name      string
	cfg       config.NodeConfig
	State     string
	SSH       config.SSHConfig
	LastSeen  time.Time
	PingFunc  agents.Pinger
	RunFunc   agents.Runner
	WakeFunc  agents.WakeSender
	AfterFunc agents.After
}

type NodeStatus struct {
	PowerState string    `json:"power_state"`
	LastSeen   time.Time `json:"last_seen"`
	Address    string    `json:"address"`
}

type DetailedStatus struct {
	NodeStatus
	MAC         string `json:"mac"`
	BootTimeout int    `json:"boot_timeout"`
}

func (n *Node) Status() *NodeStatus {
	status := &NodeStatus{
		PowerState: n.State,
		Address:    n.cfg.Address,
		LastSeen:   n.LastSeen,
	}
	return status
}

func (n *Node) GetDetailedStatus() *DetailedStatus {
	status := &DetailedStatus{
		NodeStatus:  *n.Status(),
		MAC:         n.cfg.MAC,
		BootTimeout: n.cfg.BootTimeout,
	}

	return status
}

func NewNode(name string, cfg config.NodeConfig, ssh config.SSHConfig) *Node {
	n := &Node{
		Name:  name,
		cfg:   cfg,
		State: UNKNOWN,
		SSH:   ssh,
	}
	n.PingFunc = utils.Ping
	n.RunFunc = utils.SSHRun
	n.WakeFunc = utils.Wake
	n.AfterFunc = utils.After

	return n
}

// TODO: Initialize the node, check if it's online etc.
func (n *Node) Init() {
	n.IsReachable()
	slog.Info("node initialized!", "node", n.Name, "status", n.State)
}

func (n *Node) IsReachable() {
	// status := utils.(n.cfg.Address)
	status := n.PingFunc(n.cfg.Address)

	slog.Info("health check status", "node", n.Name, "status", status)

	if status {
		n.State = ON
		n.LastSeen = time.Now()
	} else {
		n.State = OFF
	}
}

func (n *Node) FinishBooting() {
	// timer := time.NewTimer(time.Duration(n.cfg.BootTimeout) * time.Second)
	timer := n.AfterFunc(time.Duration(n.cfg.BootTimeout) * time.Second)
	slog.Info("starting boot timer", "node", n.Name)
	<-timer
	slog.Info("boot timer complete", "node", n.Name)
	n.IsReachable()

}

func (n *Node) FinishShutdown() {
	// timer := time.NewTimer(time.Duration(n.cfg.BootTimeout) * time.Second)
	timer := n.AfterFunc(time.Duration(n.cfg.BootTimeout) * time.Second)
	slog.Info("starting shutdown timer", "node", n.Name)
	<-timer
	slog.Info("shutdown timer complete", "node", n.Name)
	n.IsReachable()
}

func (n *Node) Boot(ctx context.Context) error {
	slog.Info("sending magic packet", "node", n.Name, "mac", n.cfg.MAC, "address", n.cfg.Address)

	if n.State == ON {
		slog.Info("node online, no need of boot", "node", n.Name)
		return fmt.Errorf("node online, no need of boot; node: %s; %w", n.Name, apierr.ErrConflict)
	}

	if n.State != OFF {
		slog.Info("node status unknown, skipping boot", "node", n.Name)
		return fmt.Errorf("node status unknown, skipping boot; node: %s; %w", n.Name, apierr.ErrConflict)
	}

	err := n.WakeFunc(n.cfg.MAC)

	if err != nil {
		slog.Error("failed to boot node", "node", n.Name, "error", err)
		return fmt.Errorf("failed to shutdown node; node %s; error: %v; %w", n.Name, err, apierr.ErrUpstream)
	}

	// TODO: Check for ping to start etc.
	n.State = BOOTING
	go n.FinishBooting()
	slog.Info("wol request sent", "node", n.Name)
	return nil
}

func (n *Node) Shutdown(ctx context.Context) error {
	// Skip if the host is already off.
	if n.State == OFF {
		slog.Info("node offline, no need for shutdown.", "node", n.Name)
		return fmt.Errorf("node offline, no need for shutdown; node: %s; %w", n.Name, apierr.ErrConflict)
	}

	// Don't do anything if we don't know if the host is up.
	if n.State != ON {
		slog.Info("node status unknown, skipping shutdown", "node", n.Name)
		return fmt.Errorf("node status unknown, skipping shutdown; node: %s; %w", n.Name, apierr.ErrConflict)
	}

	// Shutdown in 60 secs.
	err := n.RunFunc(ctx, n.cfg.Address, "shutdown now", n.SSH)
	// stdout, stderr, err := n.RunCommand(ctx, "shutdown now")
	if err != nil {
		slog.Error("failed to shutdown node", "node", n.Name, "error", err)
		return fmt.Errorf("failed to shutdown node; node: %s; error: %v; %w", n.Name, err, apierr.ErrUpstream)
	}

	// slog.Info("shutdown command executed", "stdout", stdout, "stderr", stderr, "error", err)

	n.State = SHUTTING
	go n.FinishShutdown()
	return nil
}
