package sentinel

import (
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

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
	LastPing bool // True if it was successfull
	State    SentinelState
	cfg      config.SentinelConfig
}

func NewSentinel(cfg *config.Config) *Sentinel {
	return &Sentinel{
		State: Normal,
		cfg:   cfg.Sentinel,
	}
}

// TODO: Implement Init(), ping and find out current sentinel status etc.
func (s *Sentinel) Init() {
}

// TODO: Implement pinging!
func (s *Sentinel) Ping() (bool, error) {
	return utils.Ping(s.cfg.Address)
}
