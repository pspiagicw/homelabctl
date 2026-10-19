# Deploying to the Raspberry Pi (falcon-control)

homelabctld runs on a Raspberry Pi 5 as your user, under a **user** systemd
unit. Nothing runs as root.

```
~/.local/bin/homelabctld                     # the daemon (static arm64 binary)
~/.config/homelabctl/config.yaml             # copied from deploy/config.yaml
~/.config/homelabctl/keys/id_ed25519         # SSH key for shutting nodes down
~/.config/systemd/user/homelabctld.service   # from deploy/systemd/
```

The Pi host defaults to `falcon-control`, an `~/.ssh/config` alias with key
auth: `ssh falcon-control` must log in without a password prompt. The make
targets use `BatchMode=yes`, so they fail instead of prompting. Override with
`PI_HOST=...`.

## One-time setup on the Pi

1. Install the unit:
   ```sh
   mkdir -p ~/.config/systemd/user
   # copy deploy/systemd/homelabctld.service into ~/.config/systemd/user/
   systemctl --user daemon-reload
   systemctl --user enable homelabctld
   ```
2. Let the service start at boot without logging in:
   ```sh
   sudo loginctl enable-linger $USER
   ```
3. Check unprivileged ping is allowed. The daemon's pings need your group in
   `ping_group_range`; otherwise all nodes show `off` and the sentinel `down`.
   ```sh
   cat /proc/sys/net/ipv4/ping_group_range   # want: 0	2147483647
   ```
   Debian 13 (trixie), which falcon-control runs, already defaults to this. On
   an OS that ships `1 0`, fix it with:
   ```sh
   echo 'net.ipv4.ping_group_range = 0 2147483647' | sudo tee /etc/sysctl.d/99-homelabctl-ping.conf
   sudo sysctl --system
   ```
4. From the laptop, push the config and the binary:
   ```sh
   make deploy-config
   make deploy
   ```
5. SSH key (later): put the private key at `~/.config/homelabctl/keys/id_ed25519`
   (mode 0600) and its public key in root's `authorized_keys` on every node.
   Until then Boot (Wake-on-LAN) works, but Shut down fails with an SSH error.

## Every release

```sh
make deploy          # build, push, restart
```

After a config change:

```sh
make deploy-config
ssh falcon-control systemctl --user restart homelabctld
```

Check on it:

```sh
ssh falcon-control systemctl --user status homelabctld
ssh falcon-control journalctl --user-unit homelabctld -f
```

## Warning: check the sentinel before restarting

If falcon-lite (the sentinel) is down when the daemon starts, it skips the
outage grace period and shuts down every running managed node about 30
seconds later. Check `curl falcon-control:3000/status` shows `"state":"Normal"`
before `make deploy` or a restart.
