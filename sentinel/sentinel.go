package sentinel

// TODO: Implement component based logging!

import (
	"context"
	"log/slog"
	"time"

	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

type SentinelState string

const (
	Normal          SentinelState = "Normal"
	OutageDetected                = "OutageDetected"
	OutageConfirmed               = "OutageConfirmed"
	Restoring                     = "Restoring"
	Restored                      = "Restored"
	Unkown                        = "Unknown"
)

// TOOD: Implement last-seen and transision timestamps
type Sentinel struct {
	State         SentinelState
	cfg           config.SentinelConfig
	GraceUpTime   int
	GraceDownTime int
	Logger        *slog.Logger
	OnOutage      func(context.Context)
	OnRestore     func(context.Context)
}

type SentinelStatus struct {
	State            string    `json:"state"`
	LastSeen         time.Time `json:"last_seen"`
	LastTransitioned time.Time `json:"last_transitioned"`
}

func NewSentinel(cfg *config.Config) *Sentinel {
	return &Sentinel{
		State:         Unkown,
		cfg:           cfg.Sentinel,
		GraceUpTime:   0,
		GraceDownTime: 0,
	}
}

func (s *Sentinel) SetFunc(onOutage, onRestore func(context.Context)) {
	s.OnOutage = onOutage
	s.OnRestore = onRestore
}

// TODO: Implement Init(), ping and find out current sentinel status etc.
func (s *Sentinel) Init() {
	pingStatus := s.Ping()

	slog.Info("sentinel status", "address", s.cfg.Address, "status", pingStatus)

	if pingStatus {
		s.State = Normal
		slog.Info("sentinel is up!")
	} else {
		s.State = OutageConfirmed
		slog.Info("sentinel is down!")
	}

	slog.Info("sentinel initialized!")
}

// TODO: Implement pinging!
func (s *Sentinel) Ping() bool {
	return utils.Ping(s.cfg.Address)
}

func (s *Sentinel) Status() *SentinelStatus {
	status := &SentinelStatus{
		State: string(s.State),
	}
	return status
}
