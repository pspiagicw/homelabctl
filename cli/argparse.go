package cli

type Options struct {
}

type CLI struct {
	Config     string `help:"Path to config file (overrides default resolution)"`
	JsonOutput bool   `help:"Output machine-readable JSON instead of a table"`
	LogLevel   string `help:"Set logging verbosity (debug, info, warn, error)"`

	Status   StatusCMD   `help:"Show current profile, override, sentinel state, and node status"`
	Nodes    NodesCMD    `help:"Manage and inspect individual nodes"`
	Boot     BootCMD     `help:"Boot a single node manually, outside of any profile"`
	Shutdown ShutdownCMD `help:"Shut down a single node manually, outside of any profile"`
	Profile  ProfileCMD  `help:"Manage and inspect power profiles"`
	Version  VersionCMD  `help:"Print build and version information"`
}
