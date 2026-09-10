package config

// ssh:
//   user: homelabctl
//   identity_file: /etc/homelabctl/keys/id_ed25519
//   port: 22
//   connection_timeout: 10
//   strict_host_key_checking: true
//   known_hosts_file: /etc/homelabctl/keys/known_hosts

type SSHConfig struct {
	User                  string `yaml:"user"`
	IdentityFile          string `yaml:"identity_file"`
	Password              string `yaml:"password"`
	Port                  int    `yaml:"port"`
	ConnectionTimeout     int    `yaml:"connection_timeout"`
	StrictHostKeyChecking bool   `yaml:"strict_host_key_checking"`
	KnownHostsFile        string `yaml:"known_hosts_file"`
}

// sentinel:
//   host: falcon-lite
//   address: 192.168.1.10
//   check_interval: 30          # how often we ping falcon-lite
//   outage_grace_period: 900    # 15 min of no response before declaring outage
//   restore_grace_period: 900   # 15 min of falcon-lite being back before WoL'ing nodes

type SentinelConfig struct {
	Host               string `yaml:"host"`
	Address            string `yaml:"address"`
	CheckInterval      int    `yaml:"check_interval"`
	OutageGracePeriod  int    `yaml:"outage_grace_period"`
	RestoreGracePeriod int    `yaml:"restore_grace_perioud"`
}

// nodes:
// falcon-heavy:
//   address: 192.168.1.12
//   mac: "b8:27:eb:00:00:02"
//   wait_after_boot: 180        # static delay before considered "ready"
//   services:
//     - jellyfin
//     - immich
//     - truenas

// TODO: Services per node, not implemented.
type NodeConfig struct {
	Address     string `yaml:"address"`
	MAC         string `yaml:"mac"`
	BootTimeout int    `yaml:"wait_after_boot"`
}

// profiles:
//   backup:
//     nodes:
//       - falcon1
//       - falcon-heavy

type ProfileConfig struct {
	Nodes []string `yaml:"nodes"`
}

// schedule:
//   entries:
//     - time: "09:00"
//       profile: backup
//     - time: "12:30"
//       profile: movie
//     - time: "23:00"
//       profile: off

type ScheduleEntry struct {
	Time    string `yaml:"time"`
	Profile string `yaml:"profile"`
}

type ScheduleConfig struct {
	Entries []ScheduleEntry `yaml:"entries"`
}

type Config struct {
	SSH      SSHConfig                `yaml:"ssh"`
	Sentinel SentinelConfig           `yaml:"sentinel"`
	Nodes    map[string]NodeConfig    `yaml:"nodes"`
	Profiles map[string]ProfileConfig `yaml:"profiles"`
	Schedule ScheduleConfig           `yaml:"schedule"`
}
