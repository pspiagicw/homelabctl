// Package schedule applies profiles at fixed times of day (the schedule:
// section of the config, in the host's local time). It only acts at those
// times: missed entries aren't replayed and nothing is applied at startup, so
// a daemon restart never boots machines on its own.
package schedule

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"sync"
	"time"

	"github.com/pspiagicw/homelabctl/agents"
	"github.com/pspiagicw/homelabctl/config"
	"github.com/pspiagicw/homelabctl/profile"
	"github.com/pspiagicw/homelabctl/utils"
)

// Trigger marks profile applies made by the scheduler.
const Trigger = "schedule"

// Applier applies a profile on the scheduler's behalf.
type Applier interface {
	ApplyScheduledProfile(ctx context.Context, name string) (*profile.Applied, error)
}

type Entry struct {
	Time    string `json:"time"` // HH:MM, local time
	Profile string `json:"profile"`

	minute int // minutes after midnight
}

// Run is the outcome of one scheduled apply.
type Run struct {
	Entry Entry     `json:"entry"`
	RanAt time.Time `json:"ran_at"`
	Error string    `json:"error,omitempty"`
}

type Status struct {
	Entries []Entry   `json:"entries"`  // sorted by time
	NextAt  time.Time `json:"next_at"`  // zero when there are no entries
	Next    *Entry    `json:"next"`     // nil when there are no entries
	LastRun *Run      `json:"last_run"` // nil until the first scheduled apply
}

type Scheduler struct {
	entries []Entry
	applier Applier

	// Injected so tests can control time.
	now   func() time.Time
	after agents.After

	mu      sync.Mutex
	next    *Entry
	nextAt  time.Time
	lastRun *Run
}

// NewScheduler parses the config's entries. Invalid times are an error;
// config validation should already have rejected them.
func NewScheduler(cfg *config.Config, a Applier) (*Scheduler, error) {
	entries := make([]Entry, 0, len(cfg.Schedule.Entries))
	for _, e := range cfg.Schedule.Entries {
		minute, err := ParseTime(e.Time)
		if err != nil {
			return nil, err
		}
		entries = append(entries, Entry{Time: e.Time, Profile: e.Profile, minute: minute})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].minute < entries[j].minute })

	s := &Scheduler{
		entries: entries,
		applier: a,
		now:     time.Now,
		after:   utils.After,
	}
	if len(entries) > 0 {
		at, next := s.nextAfter(s.now())
		s.next, s.nextAt = &next, at
	}
	return s, nil
}

// ParseTime turns "HH:MM" (24h) into minutes after midnight.
func ParseTime(s string) (int, error) {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0, fmt.Errorf("invalid schedule time %q, want HH:MM (24h): %w", s, err)
	}
	return t.Hour()*60 + t.Minute(), nil
}

// nextAfter returns the first entry strictly after from, wrapping to the next
// day after the last entry.
func (s *Scheduler) nextAfter(from time.Time) (time.Time, Entry) {
	midnight := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, from.Location())

	for day := 0; day < 2; day++ {
		for _, e := range s.entries {
			at := midnight.AddDate(0, 0, day).Add(time.Duration(e.minute) * time.Minute)
			if at.After(from) {
				return at, e
			}
		}
	}

	// Unreachable with at least one entry: every entry recurs tomorrow.
	return time.Time{}, Entry{}
}

// Watch sleeps until each entry's time and applies its profile, until ctx is
// cancelled.
func (s *Scheduler) Watch(ctx context.Context) {
	if len(s.entries) == 0 {
		slog.Info("no schedule entries, scheduler not started.")
		return
	}

	from := s.now()
	for {
		at, entry := s.nextAfter(from)
		s.mu.Lock()
		s.next, s.nextAt = &entry, at
		s.mu.Unlock()
		slog.Info("next scheduled profile", "profile", entry.Profile, "at", at)

		select {
		case <-ctx.Done():
			slog.Info("shutdown requested, stopping scheduler.")
			return
		case <-s.after(at.Sub(s.now())):
		}

		s.run(ctx, entry)
		// Continue from the entry's time, not the clock, so the same entry
		// can't fire twice if the timer is early by a moment.
		from = at
	}
}

func (s *Scheduler) run(ctx context.Context, entry Entry) {
	run := &Run{Entry: entry}
	if _, err := s.applier.ApplyScheduledProfile(ctx, entry.Profile); err != nil {
		run.Error = err.Error()
		slog.Warn("scheduled profile not applied", "profile", entry.Profile, "error", err)
	} else {
		slog.Info("scheduled profile applied", "profile", entry.Profile)
	}
	run.RanAt = s.now()

	s.mu.Lock()
	s.lastRun = run
	s.mu.Unlock()
}

// Status returns a copy of the schedule and its next and last runs.
func (s *Scheduler) Status() *Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := &Status{
		Entries: append([]Entry{}, s.entries...),
		NextAt:  s.nextAt,
	}
	if s.next != nil {
		next := *s.next
		status.Next = &next
	}
	if s.lastRun != nil {
		run := *s.lastRun
		status.LastRun = &run
	}
	return status
}
