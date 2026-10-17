package node

import (
	"context"
	"testing"

	"github.com/pspiagicw/homelabctl/config"
)

// setupNodeWithMocks creates a test node with mocked dependencies
func setupNodeWithMocks() *Node {
	cfg := config.NodeConfig{
		Address:       "192.168.1.100",
		MAC:           "00:11:22:33:44:55",
		BootTimeout:   30,
		ShutdownTimeout: 60,
	}
	sshCfg := config.SSHConfig{
		User:        "root",
		IdentityFile: "/path/to/key",
	}

	node := NewNode("test-node", cfg, sshCfg)

	// Setup default mocks
	node.WakeFunc = func(mac string) error {
		return nil // Always succeed for boot tests
	}
	
	node.PingFunc = func(address string) bool {
		return true // Default to reachable for most tests
	}
	
	node.RunFunc = func(ctx context.Context, addr, cmd string, cfg config.SSHConfig) error {
		return nil // Always succeed for shutdown tests
	}

	return node
}

// TestNodeBootWhenOff verifies boot succeeds when node is OFF 
func TestNodeBootWhenOff(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = OFF

	err := node.Boot(context.Background())

	if err != nil {
		t.Errorf("node.Boot() error = %v, wantErr false", err)
	}

	if node.State != BOOTING {
		t.Errorf("node.State = %s, want %s", node.State, BOOTING)
	}
}

// TestNodeBootWhenOn verifies boot is skipped when node is ON
func TestNodeBootWhenOn(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = ON

	err := node.Boot(context.Background())

	if err == nil {
		t.Errorf("node.Boot() error = %v, wantErr true", err)
	}

	if node.State != ON {
		t.Errorf("node.State = %s, want %s", node.State, ON)
	}
}

// TestNodeBootWhenUnknown verifies boot is skipped when state is UNKNOWN
func TestNodeBootWhenUnknown(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = UNKNOWN

	err := node.Boot(context.Background())

	if err == nil {
		t.Errorf("node.Boot() error = %v, wantErr true", err)
	}

	if node.State != UNKNOWN {
		t.Errorf("node.State = %s, want %s", node.State, UNKNOWN)
	}
}

// TestNodeFinishBooting verifies timeout mechanism completes in goroutine
func TestNodeFinishBooting(t *testing.T) {
	cfg := config.NodeConfig{
		Address:       "192.168.1.100",
		MAC:           "00:11:22:33:44:55",
		BootTimeout:   2, // Short timeout for testing
		ShutdownTimeout: 60,
	}
	sshCfg := config.SSHConfig{
		User:        "root",
		IdentityFile: "/path/to/key",
	}

	node := NewNode("test-node", cfg, sshCfg)
	node.State = BOOTING

	// Mock PingFunc to always return true (reachable)
	node.PingFunc = func(address string) bool {
		return true
	}

	done := make(chan struct{})
	
	go func() {
		node.FinishBooting()
		close(done)
	}()

	// Wait for timeout with small buffer to allow goroutine to complete
	select {
	case <-done:
		// State should have been updated to ON or OFF after IsReachable
	}

	t.Logf("FinishBooting completed, final state: %s", node.State)
	if node.State != ON {
		t.Errorf("node.State = %s, want %s", node.State, ON)
	}
}

// TestNodeShutdownWhenOn verifies shutdown succeeds when node is ON
func TestNodeShutdownWhenOn(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = ON

	err := node.Shutdown(context.Background())

	if err != nil {
		t.Errorf("node.Shutdown() error = %v, wantErr false", err)
	}

	if node.State != SHUTTING {
		t.Errorf("node.State = %s, want %s", node.State, SHUTTING)
	}
}

// TestNodeShutdownWhenOff verifies shutdown is skipped when node is OFF 
func TestNodeShutdownWhenOff(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = OFF

	err := node.Shutdown(context.Background())

	if err == nil {
		t.Errorf("node.Shutdown() error = %v, wantErr true", err)
	}

	if node.State != OFF {
		t.Errorf("node.State = %s, want %s", node.State, OFF)
	}
}

// TestNodeShutdownWhenUnknown verifies shutdown is skipped when state is UNKNOWN
func TestNodeShutdownWhenUnknown(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = UNKNOWN

	err := node.Shutdown(context.Background())

	if err == nil {
		t.Errorf("node.Shutdown() error = %v, wantErr true", err)
	}

	if node.State != UNKNOWN {
		t.Errorf("node.State = %s, want %s", node.State, UNKNOWN)
	}
}

// TestNodeFinishShutdown verifies shutdown timeout mechanism completes in goroutine
func TestNodeFinishShutdown(t *testing.T) {
	cfg := config.NodeConfig{
		Address:       "192.168.1.100",
		MAC:           "00:11:22:33:44:55",
		BootTimeout:   30,
		ShutdownTimeout: 2, // Short timeout for testing
	}
	sshCfg := config.SSHConfig{
		User:        "root",
		IdentityFile: "/path/to/key",
	}

	node := NewNode("test-node", cfg, sshCfg)
	node.State = SHUTTING

	// Mock PingFunc to always return false (not reachable - simulating shutdown)
	node.PingFunc = func(address string) bool {
		return false
	}

	done := make(chan struct{})
	
	go func() {
		node.FinishShutdown()
		close(done)
	}()

	// Wait for timeout with small buffer to allow goroutine to complete
	select {
	case <-done:
		t.Logf("FinishShutdown completed after timeout, final state: %s", node.State)
	}

	if node.State != OFF {
		t.Errorf("node.State = %s, want %s", node.State, OFF)
	}
}
