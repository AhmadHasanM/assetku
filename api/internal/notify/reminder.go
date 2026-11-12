package notify

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/db/gen"
)

var monthsID = [...]string{"Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
var monthsEN = [...]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

func formatDate(d civil.Date, months [12]string) string {
	return fmt.Sprintf("%02d %s %d", d.Day, months[d.Month-1], d.Year)
}

// RunReminders sends the maintenance reminders due on today (Asia/Jakarta
// date chosen by the caller) and returns how many notifications were created.
//
// Only the latest record of each asset counts. Each record is handled in its
// own transaction: the reminder_sent_at claim and the notification inserts
// commit together, and the claim is conditional on reminder_sent_at IS NULL so
// concurrent runs cannot double-send.
func RunReminders(ctx context.Context, pool *pgxpool.Pool, today civil.Date) (int, error) {
	cands, err := gen.New(pool).DueReminderCandidates(ctx, pgtype.Date{Time: today.In(time.UTC), Valid: true})
	if err != nil {
		return 0, err
	}
	sent := 0
	for _, c := range cands {
		n, err := sendOne(ctx, pool, c)
		sent += n
		if err != nil {
			return sent, err
		}
	}
	return sent, nil
}

func sendOne(ctx context.Context, pool *pgxpool.Pool, c gen.DueReminderCandidatesRow) (int, error) {
	next := civil.DateOf(c.NextDate.Time)
	n := 0
	err := pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		q := gen.New(tx)
		claimed, err := q.ClaimReminder(ctx, c.ID)
		if err != nil || claimed == 0 {
			return err
		}
		rows, err := q.InsertReminderNotifications(ctx, gen.InsertReminderNotificationsParams{
			TenantID: c.TenantID,
			AssetID:  uuid.NullUUID{UUID: c.AssetID, Valid: true},
			TitleID:  "Pengingat maintenance",
			TitleEn:  "Maintenance reminder",
			BodyID:   fmt.Sprintf("Maintenance %s (%s) jatuh tempo %s", c.AssetName, c.AssetTag, formatDate(next, monthsID)),
			BodyEn:   fmt.Sprintf("Maintenance for %s (%s) is due %s", c.AssetName, c.AssetTag, formatDate(next, monthsEN)),
		})
		n = int(rows)
		return err
	})
	if err != nil {
		return 0, err
	}
	return n, nil
}
