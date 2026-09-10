package node

import "github.com/pspiagicw/homelabctl/config"

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
}
