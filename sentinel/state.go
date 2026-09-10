package sentinel

import "github.com/pspiagicw/homelabctl/config"

type SentinelState int

const (
	Normal SentinelState = iota
	OutageDetected
	OutageConfirmed
	Restoring
	Restored
)

// TOOD: Implement last-seen and transision timestamps
type Sentinel struct {
	State SentinelState
	cfg   config.SentinelConfig
}

func NewSentinel(cfg config.SentinelConfig) *Sentinel {
	return &Sentinel{
		State: Normal,
		cfg:   cfg,
	}
}
