package config

import (
	"strings"
	"testing"
)

// validConfig is a complete config that passes validation.
func validConfig() *Config {
	return &Config{
		SSH: SSHConfig{User: "root", IdentityFile: "/nonexistent/key", Port: 22, ConnectionTimeout: 10},
		Sentinel: SentinelConfig{
			Host: "router", Address: "192.168.10.1",
			CheckInterval: 30, OutageGracePeriod: 600, RestoreGracePeriod: 600,
		},
		Nodes: map[string]NodeConfig{
			"falcon1": {Address: "192.168.10.44", MAC: "aa:bb:cc:00:00:44", BootTimeout: 60, ShutdownTimeout: 60},
			"desktop": {Address: "desktop.lan", MAC: "aa:bb:cc:00:00:40", BootTimeout: 60, ShutdownTimeout: 60, Unmanaged: true},
		},
		Services: ServicesConfig{
			{Name: "Jellyfin", URL: "http://192.168.10.91:8096", Node: "falcon1"},
			{Name: "PBS", URL: "https://192.168.10.42:8007"},
		},
		Profiles: map[string]ProfileConfig{
			"backup": {Nodes: []string{"falcon1"}},
			"off":    {Nodes: []string{}},
		},
		Schedule: ScheduleConfig{Entries: []ScheduleEntry{
			{Time: "09:00", Profile: "backup"},
			{Time: "23:00", Profile: "off"},
		}},
	}
}

// TestValidateAcceptsValidConfig also covers the missing identity file, which
// is only a warning.
func TestValidateAcceptsValidConfig(t *testing.T) {
	if err := Validate(validConfig()); err != nil {
		t.Errorf("Validate() error = %v", err)
	}
}

func TestValidateRejects(t *testing.T) {
	node := func(c *Config, name string, f func(*NodeConfig)) {
		n := c.Nodes[name]
		f(&n)
		c.Nodes[name] = n
	}

	tests := []struct {
		name   string
		modify func(*Config)
		want   string
	}{
		// ssh
		{"ssh user missing", func(c *Config) { c.SSH.User = "" }, "ssh: user is required"},
		{"ssh port out of range", func(c *Config) { c.SSH.Port = 0 }, "port 0 out of range"},
		{"ssh negative timeout", func(c *Config) { c.SSH.ConnectionTimeout = -1 }, "connection_timeout -1"},
		{"ssh no credentials", func(c *Config) { c.SSH.IdentityFile = "" }, "identity_file or password"},
		{"ssh strict without known_hosts", func(c *Config) { c.SSH.StrictHostKeyChecking = true }, "needs known_hosts_file"},
		// sentinel
		{"sentinel host missing", func(c *Config) { c.Sentinel.Host = "" }, "sentinel: host is required"},
		{"sentinel bad address", func(c *Config) { c.Sentinel.Address = "not a host" }, `sentinel: invalid address "not a host"`},
		{"sentinel zero interval", func(c *Config) { c.Sentinel.CheckInterval = 0 }, "check_interval must be positive"},
		{"sentinel zero outage grace", func(c *Config) { c.Sentinel.OutageGracePeriod = 0 }, "outage_grace_period must be positive"},
		{"sentinel zero restore grace", func(c *Config) { c.Sentinel.RestoreGracePeriod = 0 }, "restore_grace_period must be positive"},
		{"sentinel host is a node", func(c *Config) { c.Sentinel.Host = "falcon1" }, "must not also be under nodes"},
		{"sentinel address is a node's", func(c *Config) { c.Sentinel.Address = "192.168.10.44" }, "also used by node falcon1"},
		// nodes
		{"node bad address", func(c *Config) { node(c, "falcon1", func(n *NodeConfig) { n.Address = "" }) }, `node falcon1: invalid address ""`},
		{"node invalid IP", func(c *Config) { node(c, "falcon1", func(n *NodeConfig) { n.Address = "192.168.10.300" }) }, "invalid address"},
		{"node bad mac", func(c *Config) { node(c, "falcon1", func(n *NodeConfig) { n.MAC = "zz:zz" }) }, `invalid mac "zz:zz"`},
		{"node duplicate address", func(c *Config) { node(c, "desktop", func(n *NodeConfig) { n.Address = "192.168.10.44" }) }, "address 192.168.10.44 is also used"},
		{"node duplicate mac", func(c *Config) { node(c, "desktop", func(n *NodeConfig) { n.MAC = "AA:BB:CC:00:00:44" }) }, "is also used by node"},
		{"node zero boot wait", func(c *Config) { node(c, "falcon1", func(n *NodeConfig) { n.BootTimeout = 0 }) }, "wait_after_boot must be positive"},
		{"node zero shutdown wait", func(c *Config) { node(c, "falcon1", func(n *NodeConfig) { n.ShutdownTimeout = 0 }) }, "wait_after_shutdown must be positive"},
		// services
		{"service name missing", func(c *Config) { c.Services[0].Name = "" }, "service #1: name is required"},
		{"service name twice", func(c *Config) { c.Services[1].Name = "Jellyfin" }, "name listed twice"},
		{"service non-http url", func(c *Config) { c.Services[0].URL = "ssh://192.168.10.91" }, "must be an http(s) URL"},
		{"service unknown node", func(c *Config) { c.Services[0].Node = "nosuch" }, "service Jellyfin: unknown node nosuch"},
		// profiles
		{"profile unknown node", func(c *Config) { c.Profiles["x"] = ProfileConfig{Nodes: []string{"nosuch"}} }, "profile x: unknown node nosuch"},
		{"profile unmanaged node", func(c *Config) { c.Profiles["x"] = ProfileConfig{Nodes: []string{"desktop"}} }, "unmanaged"},
		{"profile node twice", func(c *Config) { c.Profiles["x"] = ProfileConfig{Nodes: []string{"falcon1", "falcon1"}} }, "listed twice"},
		// schedule
		{"schedule invalid time", func(c *Config) { c.Schedule.Entries[0].Time = "9am" }, `invalid time "9am"`},
		{"schedule duplicate time", func(c *Config) { c.Schedule.Entries[1].Time = "09:00" }, "time 09:00 listed twice"},
		{"schedule unknown profile", func(c *Config) { c.Schedule.Entries[0].Profile = "nosuch" }, "unknown profile nosuch"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := validConfig()
			tt.modify(c)

			err := Validate(c)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("Validate() error = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

// TestValidateReportsEveryProblem checks errors are collected, not stopped at
// the first one.
func TestValidateReportsEveryProblem(t *testing.T) {
	c := validConfig()
	c.SSH.User = ""
	c.Sentinel.CheckInterval = 0
	c.Schedule.Entries[0].Profile = "nosuch"

	err := Validate(c)
	for _, want := range []string{"ssh: user", "check_interval", "unknown profile"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("Validate() error = %v, missing %q", err, want)
		}
	}
}
