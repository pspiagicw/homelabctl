package daemon

import (
	"github.com/pspiagicw/homelabctl/node"
	"github.com/pspiagicw/homelabctl/sentinel"
)

type Response struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
}

type DaemonStatus struct {
	Sentinel *sentinel.SentinelStatus `json:"sentinel"`
	Registry *node.RegistryStatus     `json:"registry"`
}
