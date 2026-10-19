package service

import (
	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/utils"
)

type Service struct {
	PingFunc  agents.Pinger
	AfterFunc agents.After

	cfg config.ServicesConfig
}

func NewService(cfg *config.Config) *Service {
	return &Service{
		PingFunc:  utils.Ping,
		AfterFunc: utils.After,

		cfg: cfg.Services,
	}
}

type ServiceList struct {
	Services []config.ServiceConfig `json:"services"`
}

func (s *Service) GetServices() *ServiceList {
	list := &ServiceList{
		Services: s.cfg,
	}

	return list
}
