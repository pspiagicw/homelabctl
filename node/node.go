package node

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/linde12/gowol"
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

type PowerState string

const (
	UNKNOWN     PowerState = "unknown"
	OFF         PowerState = "off"
	BOOTING     PowerState = "booting"
	ON          PowerState = "on"
	UNREACHABLE PowerState = "unreachable"
)

// TODO: Implement services workflow
type Node struct {
	Name  string
	cfg   config.NodeConfig
	State PowerState
	SSH   config.SSHConfig
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
		PowerState: string(n.State),
		Address:    n.cfg.Address,
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

	return n
}

// TODO: Initialize the node, check if it's online etc.
func (n *Node) Init() {
	n.IsReachable()
	slog.Info("node initialized!", "node", n.Name, "status", n.State)
}

func (n *Node) IsReachable() {
	status := utils.Ping(n.cfg.Address)

	slog.Info("health check status", "node", n.Name, "status", status)

	if status {
		n.State = ON
	} else {
		n.State = OFF
	}
}

func (n *Node) Boot(ctx context.Context) error {
	slog.Info("sending magic packet", "node", n.Name, "mac", n.cfg.MAC, "address", n.cfg.Address)

	if n.State == ON {
		slog.Info("node online, no need of boot", "node", n.Name)
		return fmt.Errorf("node online, no need of boot; node: %s", n.Name)
	}

	if n.State != OFF {
		slog.Info("node status unknown, skipping boot", "node", n.Name)
		return fmt.Errorf("node status unknown, skipping boot; node: %s", n.Name)
	}

	packet, err := gowol.NewMagicPacket(n.cfg.MAC)

	if err != nil {
		// Log one for the server and return error for the request
		slog.Error("failed to create magic packet", "node", n.Name, "error", err)
		return fmt.Errorf("failed to create magic packet; node: %s; error: %v", n.Name, err)
	}

	err = packet.Send("255.255.255.255")
	if err != nil {
		slog.Error("failed to send magic packaet", "node", n.Name, "error", err)
		return fmt.Errorf("failed to send magic packet; node: %s; error: %v", n.Name, err)
	}

	// TODO: Check for ping to start etc.
	n.State = ON
	slog.Info("wol request sent", "node", n.Name)
	return nil
}

func (n *Node) Shutdown(ctx context.Context) error {
	// Skip if the host is already off.
	if n.State == OFF {
		slog.Info("node offline, no need for shutdown.", "node", n.Name)
		return fmt.Errorf("node offline, no need for shutdown; node: %s", n.Name)
	}

	// Don't do anything if we don't know if the host is up.
	if n.State != ON {
		slog.Info("node status unknown, skipping shutdown", "node", n.Name)
		return fmt.Errorf("node status unknown, skipping shutdown; node: %s", n.Name)
	}

	// Shutdown in 60 secs.
	stdout, stderr, err := n.RunCommand(ctx, "shutdown now")
	if err != nil {
		slog.Error("failed to shutdown node", "node", n.Name, "error", err)
		return fmt.Errorf("failed to shutdown node; node: %s; error: %v", n.Name, err)
	}

	slog.Info("shutdown command executed", "stdout", stdout, "stderr", stderr, "error", err)

	n.State = OFF
	return nil
}
