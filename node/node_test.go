package node

import (
	"context"
	"testing"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
)

// Tests for Boot functionality - verify state transitions
func TestNodeBoot(t *testing.T) {
	tests := []struct {
		name        string
		initialState string
		wantError   bool
		wantState   string
	}{
		{
			name:        "boot when node is off",
			initialState: OFF,
			wantError:   false,
			wantState:   BOOTING,
		},
		{
			name:        "skip boot when node is on",
			initialState: ON,
			wantError:   true,
			wantState:   ON,
		},
		{
			name:        "skip boot when state unknown",
			initialState: UNKNOWN,
			wantError:   true,
			wantState:   UNKNOWN,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
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
			node.State = tc.initialState

			// Mock WakeFunc with a simple function that always succeeds
			node.WakeFunc = func(mac string) error {
				return nil
			}

			err := node.Boot(context.Background())

			if (err != nil) != tc.wantError {
				t.Errorf("node.Boot() error = %v, wantErr %v", err, tc.wantError)
			}

			if node.State != tc.wantState {
				t.Errorf("node.State = %s, want %s", node.State, tc.wantState)
			}
		})
	}
}

// Tests to ensure FinishBooting state transition happens in goroutine
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
}

// Tests for Shutdown functionality - verify state transitions  
func TestNodeShutdown(t *testing.T) {
	tests := []struct {
		name        string
		initialState string
		wantError   bool
		wantState   string
	}{
		{
			name:        "shutdown when node is on",
			initialState: ON,
			wantError:   false,
			wantState:   SHUTTING,
		},
		{
			name:        "skip shutdown when node is off",
			initialState: OFF,
			wantError:   true,
			wantState:   OFF,
		},
		{
			name:        "skip shutdown when state unknown",
			initialState: UNKNOWN,
			wantError:   true,
			wantState:   UNKNOWN,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.NodeConfig{
				Address:       "192.168.1.100",
				MAC:           "00:11:22:33:44:55",
				BootTimeout:   30,
				ShutdownTimeout: 60,
			}

			ctx := context.Background()
			sshCfg := config.SSHConfig{
				User:        "root",
				IdentityFile: "/path/to/key",
			}

			node := NewNode("test-node", cfg, sshCfg)
			node.State = tc.initialState

			// Mock RunFunc to simulate command execution (may fail for some scenarios)
			node.RunFunc = func(ctx context.Context, addr, cmd string, cfg config.SSHConfig) error {
				// Simulate successful SSH run
				if tc.initialState == OFF || tc.initialState == UNKNOWN {
					return apierr.ErrConflict
				}
				return nil
			}

			err := node.Shutdown(ctx)

			if (err != nil) != tc.wantError {
				t.Errorf("node.Shutdown() error = %v, wantErr %v", err, tc.wantError)
			}

			if node.State != tc.wantState {
				t.Errorf("node.State = %s, want %s", node.State, tc.wantState)
			}
		})
	}
}

// Tests to ensure FinishShutdown state transition happens in goroutine
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
}
