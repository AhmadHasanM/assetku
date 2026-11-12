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

func (s *Scheduler) run(ctx context.Context) {
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	loc := s.Loc
	if loc == nil {
		loc = time.UTC
	}
	n, err := RunReminders(ctx, s.Pool, civil.DateOf(clock().In(loc)))
	if err != nil {
		log.Printf("notify: reminder run failed: %v", err)
		return
	}
	log.Printf("notify: reminder run sent %d notifications", n)
}

// Start blocks until ctx is cancelled; call it in a goroutine.
func (s *Scheduler) Start(ctx context.Context) {
	clock := s.Clock
	if clock == nil {
		clock = time.Now
	}
	loc := s.Loc
	if loc == nil {
		loc = time.UTC
	}
	s.run(ctx)
	for {
		now := clock()
		timer := time.NewTimer(nextRun(now, loc).Sub(now))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			s.run(ctx)
		}
	}
}
