package schedule

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/pspiagicw/homelabctl/apierr"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/profile"
)

// fakeApplier records which profiles were applied.
type fakeApplier struct {
	mu      sync.Mutex
	applied []string
	err     error
}

func (f *fakeApplier) ApplyScheduledProfile(_ context.Context, name string) (*profile.Applied, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, name)
	return &profile.Applied{Profile: name}, f.err
}

func (f *fakeApplier) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.applied...)
}

// fakeClock is a controllable time source: after() hands out a channel the
// test fires, and fire() moves the clock to the requested deadline.
type fakeClock struct {
	mu     sync.Mutex
	now    time.Time
	waits  chan time.Duration
	timers chan chan time.Time
}

func newFakeClock(now time.Time) *fakeClock {
	return &fakeClock{now: now, waits: make(chan time.Duration, 1), timers: make(chan chan time.Time, 1)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	c.waits <- d
	c.timers <- ch
	return ch
}

// fire waits for the scheduler to start waiting, advances the clock by that
// wait and releases it. It returns the wait.
func (c *fakeClock) fire(t *testing.T) time.Duration {
	t.Helper()
	var d time.Duration
	select {
	case d = <-c.waits:
	case <-time.After(time.Second):
		t.Fatal("scheduler never started waiting")
	}
	ch := <-c.timers
	c.mu.Lock()
	c.now = c.now.Add(d)
	c.mu.Unlock()
	ch <- c.Now()
	return d
}

var day = time.Date(2026, 10, 23, 8, 0, 0, 0, time.Local)

func setupScheduler(t *testing.T, a Applier, now time.Time) (*Scheduler, *fakeClock) {
	t.Helper()
	cfg := &config.Config{Schedule: config.ScheduleConfig{Entries: []config.ScheduleEntry{
		{Time: "23:00", Profile: "off"},
		{Time: "09:00", Profile: "backup"},
		{Time: "12:30", Profile: "movie"},
	}}}
	s, err := NewScheduler(cfg, a)
	if err != nil {
		t.Fatalf("NewScheduler() error = %v", err)
	}
	clock := newFakeClock(now)
	s.now, s.after = clock.Now, clock.After
	return s, clock
}

func TestParseTime(t *testing.T) {
	tests := []struct {
		in      string
		want    int
		wantErr bool
	}{
		{"09:00", 540, false},
		{"23:59", 1439, false},
		{"00:00", 0, false},
		{"24:00", 0, true},
		{"9am", 0, true},
		{"", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseTime(tt.in)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Errorf("ParseTime(%q) = %d, %v; want %d, err %v", tt.in, got, err, tt.want, tt.wantErr)
		}
	}
}

func TestNextAfter(t *testing.T) {
	s, _ := setupScheduler(t, &fakeApplier{}, day)

	tests := []struct {
		from     time.Time
		wantAt   time.Time
		wantProf string
	}{
		{day, day.Add(time.Hour), "backup"},                                  // 08:00 -> 09:00
		{day.Add(time.Hour), day.Add(4*time.Hour + 30*time.Minute), "movie"}, // exactly 09:00 -> 12:30
		{day.Add(15 * time.Hour), day.Add(25 * time.Hour), "backup"},         // 23:00 -> next day's 09:00
	}
	for _, tt := range tests {
		at, e := s.nextAfter(tt.from)
		if !at.Equal(tt.wantAt) || e.Profile != tt.wantProf {
			t.Errorf("nextAfter(%v) = %v %s, want %v %s", tt.from, at, e.Profile, tt.wantAt, tt.wantProf)
		}
	}
}

func TestStatusShowsNextBeforeWatch(t *testing.T) {
	s, _ := setupScheduler(t, &fakeApplier{}, day)

	status := s.Status()
	if status.Next == nil || status.Next.Profile != "backup" {
		t.Errorf("Next = %+v, want backup", status.Next)
	}
	if len(status.Entries) != 3 || status.Entries[0].Time != "09:00" {
		t.Errorf("Entries = %+v, want sorted by time", status.Entries)
	}
}

func TestWatchAppliesEntriesInOrder(t *testing.T) {
	a := &fakeApplier{}
	s, clock := setupScheduler(t, a, day)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Watch(ctx)

	if d := clock.fire(t); d != time.Hour {
		t.Errorf("first wait = %v, want 1h until 09:00", d)
	}
	clock.fire(t) // 12:30, waits until the next run is recorded
	clock.fire(t) // 23:00

	// The scheduler is now waiting for tomorrow's 09:00.
	select {
	case d := <-clock.waits:
		if d != 10*time.Hour {
			t.Errorf("wait after 23:00 = %v, want 10h", d)
		}
	case <-time.After(time.Second):
		t.Fatal("scheduler didn't wait for the next day")
	}
	if got := a.calls(); len(got) != 3 || got[0] != "backup" || got[1] != "movie" || got[2] != "off" {
		t.Errorf("applied %v, want [backup movie off]", got)
	}
	if last := s.Status().LastRun; last == nil || last.Entry.Profile != "off" {
		t.Errorf("LastRun = %+v, want off", last)
	}
}

func TestWatchRecordsRefusedApply(t *testing.T) {
	a := &fakeApplier{err: errors.Join(errors.New("sentinel not Normal"), apierr.ErrConflict)}
	s, clock := setupScheduler(t, a, day)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.Watch(ctx)

	clock.fire(t)
	<-clock.waits // next wait started: the run is recorded

	last := s.Status().LastRun
	if last == nil || last.Error == "" {
		t.Errorf("LastRun = %+v, want the refusal recorded", last)
	}
}

func TestNoEntriesWatchReturns(t *testing.T) {
	s, err := NewScheduler(&config.Config{}, &fakeApplier{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		s.Watch(context.Background())
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Watch with no entries didn't return")
	}
}

func TestInvalidTimeRejected(t *testing.T) {
	cfg := &config.Config{Schedule: config.ScheduleConfig{Entries: []config.ScheduleEntry{{Time: "25:00", Profile: "movie"}}}}
	if _, err := NewScheduler(cfg, &fakeApplier{}); err == nil {
		t.Error("NewScheduler accepted 25:00")
	}
}
