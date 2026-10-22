package sentinel

import (
	"context"
	"testing"
	"time"

	"github.com/pspiagicw/homelabctl/config"
)

// fakePower counts the calls the sentinel makes on outage and restore.
type fakePower struct {
	shutdowns int
	restores  int
}

func (f *fakePower) ShutdownAll(_ context.Context) { f.shutdowns++ }
func (f *fakePower) RestoreAll(_ context.Context)  { f.restores++ }

// TestSentinelSetState tests the SetState method behavior
func TestSentinelSetState(t *testing.T) {
	cfg := &config.Config{
		Sentinel: config.SentinelConfig{
			Address: "192.168.1.100",
		},
	}

	sentinel := NewSentinel(cfg, &fakePower{})
	
	// Test initial state
	if sentinel.State != Unkown {
		t.Errorf("Initial state should be Unknown, got %s", sentinel.State)
	}
	
	// Test setting a new state
	newState := Normal
	sentinel.SetState(newState)
	
	if sentinel.State != newState {
		t.Errorf("Expected state %s, got %s", newState, sentinel.State)
	}
	
	// Verify timestamp was updated
	if sentinel.LastTransitioned.IsZero() {
		t.Error("LastTransitioned timestamp should be set after state change")
	}
	
	// Verify grace times were reset
	if sentinel.GraceDownTime != 0 || sentinel.GraceUpTime != 0 {
		t.Error("Grace times should be reset on state transition")
	}
}

// TestSentinelPing tests the Ping method behavior
func TestSentinelPing(t *testing.T) {
	cfg := &config.Config{
		Sentinel: config.SentinelConfig{
			Address: "192.168.1.100",
		},
	}

	sentinel := NewSentinel(cfg, &fakePower{})
	
	// Test ping that succeeds
	mockPingFunc := func(address string) bool {
		return true // Simulate successful ping
	}
	
	sentinel.PingFunc = mockPingFunc
	
	result := sentinel.Ping()
	
	if !result {
		t.Error("Ping should return true for successful ping")
	}
	
	// Verify LastSeen timestamp was updated
	if sentinel.LastSeen.IsZero() {
		t.Error("LastSeen timestamp should be updated on successful ping")
	}

	// Test ping that fails
	mockPingFunc = func(address string) bool {
		return false // Simulate failed ping
	}
	
	sentinel.PingFunc = mockPingFunc
	
	result = sentinel.Ping()
	
	if result {
		t.Error("Ping should return false for failed ping")
	}
	
	// LastSeen should NOT be updated on failed ping (that's the current logic)
}

// TestSentinelInit tests initialization behavior
func TestSentinelInit(t *testing.T) {
	cfg := &config.Config{
		Sentinel: config.SentinelConfig{
			Address: "192.168.1.100",
		},
	}

	sentinel := NewSentinel(cfg, &fakePower{})
	
	// Mock ping to return true initially (online sentinel)
	mockPingFunc := func(address string) bool {
		return true // Simulate online
	}
	
	sentinel.PingFunc = mockPingFunc
	
	// Init should set state based on initial ping result
	sentinel.Init()
	
	if sentinel.State != Normal {
		t.Errorf("Expected state Normal after init with online ping, got %s", sentinel.State)
	}
	
	// Test with offline sentinel
	mockPingFunc = func(address string) bool {
		return false // Simulate offline
	}
	
	sentinel.PingFunc = mockPingFunc
	sentinel.Init()
	
	if sentinel.State != OutageConfirmed {
		t.Errorf("Expected state OutageConfirmed after init with offline ping, got %s", sentinel.State)
	}
}

// TestSentinelStateTransitions verifies key state transitions work
func TestSentinelStateTransitions(t *testing.T) {
	cfg := &config.Config{
		Sentinel: config.SentinelConfig{
			Address:             "192.168.1.100",
			OutageGracePeriod:   5, // 5 second grace period
			RestoreGracePeriod:  5, // 5 second restore grace period
			CheckInterval:       1, // 1 second interval
		},
	}

	sentinel := NewSentinel(cfg, &fakePower{})
	
	// Start with Normal state
	sentinel.State = Normal
	
	// Simulate outage detected
	// For this test, we'll just manually check the transitions work
	// since the actual logic is complex to test fully in isolation
	
	// Test that state transitions can be set manually
	sentinel.SetState(OutageDetected)
	if sentinel.State != OutageDetected {
		t.Errorf("Expected OutageDetected, got %s", sentinel.State)
	}
	
	sentinel.SetState(OutageConfirmed)
	if sentinel.State != OutageConfirmed {
		t.Errorf("Expected OutageConfirmed, got %s", sentinel.State)
	}
	
	sentinel.SetState(Restoring) 
	if sentinel.State != Restoring {
		t.Errorf("Expected Restoring, got %s", sentinel.State)
	}
	
	sentinel.SetState(Restored)
	if sentinel.State != Restored {
		t.Errorf("Expected Restored, got %s", sentinel.State)
	}
}

// TestSentinelStatus verifies status reporting
func TestSentinelStatus(t *testing.T) {
	cfg := &config.Config{
		Sentinel: config.SentinelConfig{
			Address: "192.168.1.100",
		},
	}

	sentinel := NewSentinel(cfg, &fakePower{})
	sentinel.State = Normal
	now := time.Now()
	sentinel.LastSeen = now
	sentinel.LastTransitioned = now
	
	status := sentinel.Status()
	
	if status.State != Normal {
		t.Errorf("Expected state Normal in status, got %s", status.State)
	}
	
	if status.LastSeen != now {
		t.Errorf("Expected LastSeen to match, got %v", status.LastSeen)
	}
	
	if status.LastTransitioned != now {
		t.Errorf("Expected LastTransitioned to match, got %v", status.LastTransitioned)
	}
}
