package node

import (
	"context"
	"testing"
	"time"

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
	node.PingFunc = func(address string) bool {
		return false
	}

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

// TestNodeBootWhenBooting verifies boot is skipped, without a ping, while already BOOTING
func TestNodeBootWhenBooting(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = BOOTING
	node.PingFunc = func(address string) bool {
		t.Errorf("PingFunc called while BOOTING")
		return true
	}

	err := node.Boot(context.Background())

	if err == nil {
		t.Errorf("node.Boot() error = %v, wantErr true", err)
	}

	if node.State != BOOTING {
		t.Errorf("node.State = %s, want %s", node.State, BOOTING)
	}
}

// TestNodeBootWhenUnknownAndOff verifies an UNKNOWN state is resolved by ping before booting
func TestNodeBootWhenUnknownAndOff(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = UNKNOWN
	node.PingFunc = func(address string) bool {
		return false
	}

	err := node.Boot(context.Background())

	if err != nil {
		t.Errorf("node.Boot() error = %v, wantErr false", err)
	}

	if node.State != BOOTING {
		t.Errorf("node.State = %s, want %s", node.State, BOOTING)
	}
}

// TestNodeBootWhenStaleOn verifies boot proceeds when a recorded ON node is actually off
func TestNodeBootWhenStaleOn(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = ON
	node.PingFunc = func(address string) bool {
		return false
	}

	err := node.Boot(context.Background())

	if err != nil {
		t.Errorf("node.Boot() error = %v, wantErr false", err)
	}

	if node.State != BOOTING {
		t.Errorf("node.State = %s, want %s", node.State, BOOTING)
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
	node.PingFunc = func(address string) bool {
		return false
	}

	err := node.Shutdown(context.Background())

	if err == nil {
		t.Errorf("node.Shutdown() error = %v, wantErr true", err)
	}

	if node.State != OFF {
		t.Errorf("node.State = %s, want %s", node.State, OFF)
	}
}

// TestNodeShutdownWhenShutting verifies shutdown is skipped, without a ping, while already SHUTTING
func TestNodeShutdownWhenShutting(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = SHUTTING
	node.PingFunc = func(address string) bool {
		t.Errorf("PingFunc called while SHUTTING")
		return false
	}

	err := node.Shutdown(context.Background())

	if err == nil {
		t.Errorf("node.Shutdown() error = %v, wantErr true", err)
	}

	if node.State != SHUTTING {
		t.Errorf("node.State = %s, want %s", node.State, SHUTTING)
	}
}

// TestNodeShutdownWhenStaleOff verifies shutdown proceeds when a recorded OFF node is actually on
func TestNodeShutdownWhenStaleOff(t *testing.T) {
	node := setupNodeWithMocks()
	node.State = OFF
	node.PingFunc = func(address string) bool {
		return true
	}

	err := node.Shutdown(context.Background())

	if err != nil {
		t.Errorf("node.Shutdown() error = %v, wantErr false", err)
	}

	if node.State != SHUTTING {
		t.Errorf("node.State = %s, want %s", node.State, SHUTTING)
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

// TestRegistryShutdownAllSkipsUnmanaged verifies the outage shutdown leaves unmanaged nodes alone
func TestRegistryShutdownAllSkipsUnmanaged(t *testing.T) {
	ran := map[string]int{}
	newNode := func(name, addr string, unmanaged bool) *Node {
		n := NewNode(name, config.NodeConfig{Address: addr, Unmanaged: unmanaged}, config.SSHConfig{})
		n.State = ON
		n.PingFunc = func(string) bool { return true }
		n.RunFunc = func(_ context.Context, addr, _ string, _ config.SSHConfig) error {
			ran[addr]++
			return nil
		}
		// Never fires, so the shutdown timer goroutine doesn't change state mid-test.
		n.AfterFunc = func(time.Duration) <-chan time.Time { return make(chan time.Time) }
		return n
	}
	r := &Registry{Nodes: map[string]*Node{
		"server":  newNode("server", "192.168.10.20", false),
		"desktop": newNode("desktop", "192.168.10.40", true),
	}}

	r.ShutdownAll(context.Background())

	if ran["192.168.10.20"] != 1 {
		t.Errorf("managed node shutdown calls = %d, want 1", ran["192.168.10.20"])
	}
	if ran["192.168.10.40"] != 0 {
		t.Errorf("unmanaged node shutdown calls = %d, want 0", ran["192.168.10.40"])
	}
	if r.Nodes["desktop"].State != ON {
		t.Errorf("unmanaged node state = %s, want %s", r.Nodes["desktop"].State, ON)
	}
}
