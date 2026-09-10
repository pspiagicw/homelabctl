package node

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pspiagicw/homelabctl/config"
	"golang.org/x/crypto/ssh"
)

// TODO: Implement SSH key authentication.
// TODO: Implement host key implementation
func NewSSHClient(cfg config.SSHConfig, host string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			ssh.Password(cfg.Password),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host, config)
	if err != nil {
		return nil, fmt.Errorf("error dialing ssh connection: %v", err)
	}

	return client, nil
}

func (n *Node) RunCommand(ctx context.Context, cmd string) (stdout, stderr string, err error) {
	client, err := NewSSHClient(n.SSH, n.cfg.Address)
	if err != nil {
		return "", "", fmt.Errorf("error creating ssh client: %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", "", fmt.Errorf("error creating new ssh session: %v", err)
	}

	defer session.Close()

	var b bytes.Buffer
	var e bytes.Buffer
	session.Stdout = &b
	session.Stderr = &e

	if err := session.Run(cmd); err != nil {
		return "", "", fmt.Errorf("failed to run cmd: %v", err)
	}

	return stdout, stderr, nil
}

func (n *Node) Shutdown(ctx context.Context) error {
	_, _, err := n.RunCommand(ctx, "shutdown now")
	if err != nil {
		return fmt.Errorf("failed to shutdown node: %v", err)
	}

	return nil
}
