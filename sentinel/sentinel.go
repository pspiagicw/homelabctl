package sentinel

import (
	"fmt"
	"log/slog"
	"time"

	probing "github.com/prometheus-community/pro-bing"
	"github.com/pspiagicw/homelabctl/config"
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
	Pinger   *probing.Pinger
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
func (s *Sentinel) pingerInit() error {
	if s.Pinger == nil {
		pinger, err := probing.NewPinger(s.cfg.Address)
		if err != nil {
			return fmt.Errorf("error initializing pro-bing: %v", err)
		}
		s.Pinger = pinger
	}

	return nil
}

// TODO: Implement pinging!
func (s *Sentinel) Ping() (bool, error) {
	pinger, err := probing.NewPinger(s.cfg.Address)
	if err != nil {
		return false, err
	}

	pinger.Count = 1
	pinger.Timeout = 3 * time.Second

	if err = pinger.Run(); err != nil {
		return false, err
	}

	stats := pinger.Statistics()
	slog.Info("Statistics", "stat", stats)

	return stats.PacketsRecv > 0, nil
	// slog.Info("Staring Ping")
	// err := s.pingerInit()
	// if err != nil {
	// 	s.LastPing = false
	// }
	//
	// s.Pinger.Count = 1
	// err = s.Pinger.Run()
	// if err != nil {
	// 	s.LastPing = false
	// }
	//
	// stats := s.Pinger.Statistics()
	// slog.Info("data", stats)
	// if stats.PacketLoss != 0 {
	// 	s.LastPing = false
	// } else {
	// 	s.LastPing = true
	// }
	// slog.Warn("Ping finished")
}
