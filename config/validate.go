package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"regexp"
	"time"
)

// Validate checks every section of the config and returns all problems found,
// joined. config.New exits on a validation error, so a bad config fails at
// startup instead of when a node, profile or schedule entry is first used.
//
// A missing SSH identity file is only a warning: the daemon may start before
// keys are provisioned, and booting doesn't need SSH.
func Validate(c *Config) error {
	var errs []error
	errs = append(errs, validateSSH(c.SSH)...)
	errs = append(errs, validateSentinel(c)...)
	errs = append(errs, validateNodes(c.Nodes)...)
	errs = append(errs, validateServices(c)...)
	errs = append(errs, validateProfiles(c)...)
	errs = append(errs, validateSchedule(c)...)
	return errors.Join(errs...)
}

func validateSSH(s SSHConfig) []error {
	var errs []error

	if s.User == "" {
		errs = append(errs, errors.New("ssh: user is required"))
	}
	if s.Port < 1 || s.Port > 65535 {
		errs = append(errs, fmt.Errorf("ssh: port %d out of range 1-65535", s.Port))
	}
	if s.ConnectionTimeout < 0 {
		errs = append(errs, fmt.Errorf("ssh: connection_timeout %d is negative", s.ConnectionTimeout))
	}
	if s.IdentityFile == "" && s.Password == "" {
		errs = append(errs, errors.New("ssh: identity_file or password is required"))
	}
	if s.StrictHostKeyChecking && s.KnownHostsFile == "" {
		errs = append(errs, errors.New("ssh: strict_host_key_checking needs known_hosts_file"))
	}
	if s.IdentityFile != "" {
		if f, err := os.Open(s.IdentityFile); err != nil {
			slog.Warn("ssh identity file not readable; shutting nodes down will fail until it exists",
				"file", s.IdentityFile, "error", err)
		} else {
			f.Close()
		}
	}

	return errs
}

func validateSentinel(c *Config) []error {
	var errs []error
	s := c.Sentinel

	if s.Host == "" {
		errs = append(errs, errors.New("sentinel: host is required"))
	}
	if !validHost(s.Address) {
		errs = append(errs, fmt.Errorf("sentinel: invalid address %q", s.Address))
	}
	if s.CheckInterval <= 0 {
		errs = append(errs, fmt.Errorf("sentinel: check_interval must be positive, got %d", s.CheckInterval))
	}
	if s.OutageGracePeriod <= 0 {
		errs = append(errs, fmt.Errorf("sentinel: outage_grace_period must be positive, got %d", s.OutageGracePeriod))
	}
	if s.RestoreGracePeriod <= 0 {
		errs = append(errs, fmt.Errorf("sentinel: restore_grace_period must be positive, got %d", s.RestoreGracePeriod))
	}

	// The sentinel is a power probe on mains, never a managed node.
	if _, ok := c.Nodes[s.Host]; ok && s.Host != "" {
		errs = append(errs, fmt.Errorf("sentinel: host %s must not also be under nodes:", s.Host))
	}
	for name, n := range c.Nodes {
		if s.Address != "" && n.Address == s.Address {
			errs = append(errs, fmt.Errorf("sentinel: address %s is also used by node %s", s.Address, name))
		}
	}

	return errs
}

func validateNodes(nodes map[string]NodeConfig) []error {
	var errs []error
	addresses := map[string]string{}
	macs := map[string]string{}

	for name, n := range nodes {
		if name == "" {
			errs = append(errs, errors.New("nodes: a node has an empty name"))
		}
		if !validHost(n.Address) {
			errs = append(errs, fmt.Errorf("node %s: invalid address %q", name, n.Address))
		} else if other, dup := addresses[n.Address]; dup {
			errs = append(errs, fmt.Errorf("node %s: address %s is also used by node %s", name, n.Address, other))
		}
		addresses[n.Address] = name

		if mac, err := net.ParseMAC(n.MAC); err != nil {
			errs = append(errs, fmt.Errorf("node %s: invalid mac %q", name, n.MAC))
		} else if other, dup := macs[mac.String()]; dup {
			errs = append(errs, fmt.Errorf("node %s: mac %s is also used by node %s", name, n.MAC, other))
		} else {
			macs[mac.String()] = name
		}

		// Zero waits make the post-boot/shutdown ping happen immediately,
		// which reads the wrong state (a shutting-down node still answers).
		if n.BootTimeout <= 0 {
			errs = append(errs, fmt.Errorf("node %s: wait_after_boot must be positive, got %d", name, n.BootTimeout))
		}
		if n.ShutdownTimeout <= 0 {
			errs = append(errs, fmt.Errorf("node %s: wait_after_shutdown must be positive, got %d", name, n.ShutdownTimeout))
		}
	}

	return errs
}

func validateServices(c *Config) []error {
	var errs []error
	names := map[string]bool{}

	for i, s := range c.Services {
		label := s.Name
		if label == "" {
			label = fmt.Sprintf("#%d", i+1)
			errs = append(errs, fmt.Errorf("service %s: name is required", label))
		} else if names[s.Name] {
			errs = append(errs, fmt.Errorf("service %s: name listed twice", label))
		}
		names[s.Name] = true

		// Health checks are HTTP(S) requests to the URL.
		if u, err := url.Parse(s.URL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			errs = append(errs, fmt.Errorf("service %s: url %q must be an http(s) URL", label, s.URL))
		}
		if s.Node != "" {
			if _, ok := c.Nodes[s.Node]; !ok {
				errs = append(errs, fmt.Errorf("service %s: unknown node %s", label, s.Node))
			}
		}
	}

	return errs
}

func validateProfiles(c *Config) []error {
	var errs []error

	for name, p := range c.Profiles {
		if name == "" {
			errs = append(errs, errors.New("profiles: a profile has an empty name"))
		}
		seen := map[string]bool{}
		for _, n := range p.Nodes {
			node, ok := c.Nodes[n]
			switch {
			case !ok:
				errs = append(errs, fmt.Errorf("profile %s: unknown node %s", name, n))
			case node.Unmanaged:
				errs = append(errs, fmt.Errorf("profile %s: node %s is unmanaged and can't be in a profile", name, n))
			}
			if seen[n] {
				errs = append(errs, fmt.Errorf("profile %s: node %s listed twice", name, n))
			}
			seen[n] = true
		}
	}

	return errs
}

func validateSchedule(c *Config) []error {
	var errs []error
	times := map[string]bool{}

	for _, e := range c.Schedule.Entries {
		if _, err := time.Parse("15:04", e.Time); err != nil {
			errs = append(errs, fmt.Errorf("schedule: invalid time %q, want HH:MM (24h)", e.Time))
		}
		if times[e.Time] {
			errs = append(errs, fmt.Errorf("schedule: time %s listed twice", e.Time))
		}
		times[e.Time] = true
		if _, ok := c.Profiles[e.Profile]; !ok {
			errs = append(errs, fmt.Errorf("schedule %s: unknown profile %s", e.Time, e.Profile))
		}
	}

	return errs
}

// hostname matches RFC 1123 hostnames: dot-separated labels of letters,
// digits and inner hyphens.
var hostname = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)

// numeric matches dotted digits, which must be a valid IP, not a hostname.
var numeric = regexp.MustCompile(`^[0-9.]+$`)

// validHost reports whether s is an IP address or a hostname.
func validHost(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	return !numeric.MatchString(s) && len(s) <= 253 && hostname.MatchString(s)
}
