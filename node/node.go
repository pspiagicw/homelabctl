package node

import (
	"context"
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

func (n *Node) Status() *NodeStatus {
	status := &NodeStatus{
		PowerState: string(n.State),
		Address:    n.cfg.Address,
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

func (n *Node) Boot(ctx context.Context) bool {
	slog.Info("sending magic packet", "node", n.Name, "mac", n.cfg.MAC, "address", n.cfg.Address)
	packet, err := gowol.NewMagicPacket(n.cfg.MAC)

	if err != nil {
		slog.Error("failed to create magic packet", "node", n.Name, "error", err)
		return false
	}

	err = packet.Send("255.255.255.255")
	if err != nil {
		slog.Error("failed to send magic packaet", "node", n.Name, "error", err)
		return false
	}

	// TODO: Check for ping to start etc.
	n.State = ON
	slog.Info("wol request sent", "node", n.Name)
	return true
}
