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
4. SSH key for shutting nodes down: create it on the Pi, without a passphrase
   (the daemon can't type one), and authorise it on every managed node:
   ```sh
   mkdir -p ~/.config/homelabctl/keys && chmod 700 ~/.config/homelabctl/keys
   ssh-keygen -t ed25519 -N "" -C "homelabctld@falcon-control" \
     -f ~/.config/homelabctl/keys/id_ed25519
   # append ~/.config/homelabctl/keys/id_ed25519.pub to root's
   # ~/.ssh/authorized_keys on each managed node
   ```
   Without it, Boot (Wake-on-LAN) works but every shutdown fails: the Shut
   down button, profile switching, the auto-off, and the outage shutdown.
   `identity_file` in the config must be the absolute path (`~` isn't expanded).
5. From the laptop, push the config and the binary:
   ```sh
   make deploy-config
   make deploy
   ```

## Every release

```sh
make deploy          # build, push, restart
```

After a config change:

```sh
make deploy-config
ssh falcon-control systemctl --user restart homelabctld
```

The config is validated at startup. If it's invalid, the service exits and
every problem is in the journal (see below); fix `deploy/config.yaml` and push
it again.

Check on it:

```sh
ssh falcon-control systemctl --user status homelabctld
ssh falcon-control journalctl --user-unit homelabctld -f
ssh falcon-control '~/.local/bin/homelabctld --version'
```

The dashboard is at `http://falcon-control:3000/ui/`. Its header shows the
running version.

## The schedule is live

Once deployed, the daemon applies the `schedule:` entries every day: profiles
boot their nodes and shut down the others. To test a build on another machine
without fighting the Pi, point its config's sentinel at `127.0.0.1` and don't
leave it running across a schedule entry: two daemons would both apply
profiles.

## Docker services on the Pi

Containers on a Docker **macvlan** network get their own LAN IPs, but Linux
blocks traffic between a macvlan parent and its children, so the Pi itself
can't reach them. Their service tiles will always show **down**. Either don't
list them, use the host or bridge network, or add a macvlan "shim" interface
on the Pi.

## Warning: check the sentinel before restarting

If falcon-lite (the sentinel) is down when the daemon starts, it skips the
outage grace period and shuts down every running managed node about 30
seconds later. Check `curl falcon-control:3000/status` shows `"state":"Normal"`
before `make deploy` or a restart.
