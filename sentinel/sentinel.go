package sentinel

// TODO: Implement component based logging!

import (
	"log/slog"
	"time"

	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/backend"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

type SentinelState string

const (
	Normal          = "Normal"
	OutageDetected  = "OutageDetected"
	OutageConfirmed = "OutageConfirmed"
	Restoring       = "Restoring"
	Restored        = "Restored"
	Unkown          = "Unknown"
)

type Sentinel struct {
	State            string
	cfg              config.SentinelConfig
	GraceUpTime      int
	GraceDownTime    int
	Logger           *slog.Logger
	Power            backend.Power // shuts down / restores nodes on outage and recovery
	LastSeen         time.Time
	LastTransitioned time.Time

	AfterFunc agents.After
	PingFunc  agents.Pinger

	Log *slog.Logger
}

type SentinelStatus struct {
	State            string    `json:"state"`
	LastSeen         time.Time `json:"last_seen"`
	LastTransitioned time.Time `json:"last_transitioned"`
}

func NewSentinel(cfg *config.Config, power backend.Power) *Sentinel {
	return &Sentinel{
		State:         Unkown,
		cfg:           cfg.Sentinel,
		Power:         power,
		GraceUpTime:   0,
		GraceDownTime: 0,

		AfterFunc: utils.After,
		PingFunc:  utils.Ping,
	}

}

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

func (s *Sentinel) Ping() bool {
	// status := utils.Ping(s.cfg.Address)
	status := s.PingFunc(s.cfg.Address)
	if status {
		s.LastSeen = time.Now()
	}

	return status
}

func (s *Sentinel) Status() *SentinelStatus {
	status := &SentinelStatus{
		State:            s.State,
		LastSeen:         s.LastSeen,
		LastTransitioned: s.LastTransitioned,
	}
	return status
}
