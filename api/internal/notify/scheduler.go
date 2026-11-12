package notify

import (
	"context"
	"log"
	"time"
	_ "time/tzdata" // Asia/Jakarta must resolve on minimal containers.

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5/pgxpool"
)

const runHour = 7

// Scheduler runs RunReminders once at start and then daily at 07:00 in Loc.
type Scheduler struct {
	Pool  *pgxpool.Pool
	Loc   *time.Location
	Clock func() time.Time // defaults to time.Now
}

// nextRun returns the first 07:00 in loc strictly after now.
func nextRun(now time.Time, loc *time.Location) time.Time {
	n := now.In(loc)
	t := time.Date(n.Year(), n.Month(), n.Day(), runHour, 0, 0, 0, loc)
	if !t.After(n) {
		t = time.Date(n.Year(), n.Month(), n.Day()+1, runHour, 0, 0, 0, loc)
	}
	return t
}

// defaults returns the clock and location with nil values replaced.
func (s *Scheduler) defaults() (func() time.Time, *time.Location) {
	clock, loc := s.Clock, s.Loc
	if clock == nil {
		clock = time.Now
	}
	if loc == nil {
		loc = time.UTC
	}
	return clock, loc
}

func (s *Scheduler) run(ctx context.Context, now time.Time, loc *time.Location) {
	n, err := RunReminders(ctx, s.Pool, civil.DateOf(now.In(loc)))
	if err != nil {
		log.Printf("notify: reminder run finished with errors (sent %d): %v", n, err)
		return
	}
	log.Printf("notify: reminder run sent %d notifications", n)
}

// Start blocks until ctx is cancelled; call it in a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	clock, loc := s.defaults()
	s.run(ctx, clock(), loc)
	for {
		now := clock()
		timer := time.NewTimer(nextRun(now, loc).Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.run(ctx, clock(), loc)
		}
	}
}
