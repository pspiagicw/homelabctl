package node

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/pspiagicw/homelabctl/config"
	"golang.org/x/crypto/ssh"
)

// TODO: Implement SSH key authentication.
// TODO: Implement host key implementation
func NewSSHClient(cfg config.SSHConfig, host string) (*ssh.Client, error) {
	authMethod, err := publicKeyAuth(cfg.IdentityFile)
	if err != nil {
		return nil, fmt.Errorf("error loading identity file: %v", err)
	}

	config := &ssh.ClientConfig{
		User: cfg.User,
		Auth: []ssh.AuthMethod{
			authMethod,
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", host+":22", config)
	if err != nil {
		return nil, fmt.Errorf("error dialing ssh connection: %v", err)
	}

	return client, nil
}
func publicKeyAuth(path string) (ssh.AuthMethod, error) {
	keyBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading identity file: %v", err)
	}

	signer, err := ssh.ParsePrivateKey(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing private key: %v", err)
	}

	return ssh.PublicKeys(signer), nil

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
	slog.Info("ssh connected", "node", n.Name, "address", n.cfg.Address)

	if err := session.Run(cmd); err != nil {
		return "", "", fmt.Errorf("failed to run cmd: %v", err)
	}

	return stdout, stderr, nil
}
