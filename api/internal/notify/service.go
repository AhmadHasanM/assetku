// Package notify implements in-app notifications and the daily maintenance
// reminder job.
package notify

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
)

// Item is one notification in the user's locale.
type Item struct {
	ID        int64      `json:"id"`
	Type      string     `json:"type"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	AssetID   *uuid.UUID `json:"asset_id"`
	ReadAt    *time.Time `json:"read_at"`
	CreatedAt time.Time  `json:"created_at"`
}

// List is the GET /api/notifications response. Unread counts every unread
// notification of the user, not only those among the returned items.
type List struct {
	Unread int    `json:"unread"`
	Items  []Item `json:"items"`
}

type Service struct {
	pool *pgxpool.Pool
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service { return &Service{pool: pool, auth: a} }

// List returns the latest 20 notifications of the user; locale "en" selects
// the English text, anything else Indonesian.
func (s *Service) List(ctx context.Context, userID uuid.UUID, locale string) (List, error) {
	q := gen.New(s.pool)
	rows, err := q.ListNotifications(ctx, userID)
	if err != nil {
		return List{}, err
	}
	unread, err := q.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return List{}, err
	}
	out := List{Unread: int(unread), Items: make([]Item, 0, len(rows))}
	for _, r := range rows {
		it := Item{ID: r.ID, Type: r.Type, Title: r.TitleID, Body: r.BodyID, ReadAt: r.ReadAt, CreatedAt: r.CreatedAt}
		if locale == "en" {
			it.Title, it.Body = r.TitleEn, r.BodyEn
		}
		if r.AssetID.Valid {
			id := r.AssetID.UUID
			it.AssetID = &id
		}
		out.Items = append(out.Items, it)
	}
	return out, nil
}

// MarkRead marks one of the user's notifications read; false when it does
// not exist or belongs to someone else.
func (s *Service) MarkRead(ctx context.Context, tenantID, userID uuid.UUID, id int64) (bool, error) {
	n, err := gen.New(s.pool).MarkNotificationRead(ctx, gen.MarkNotificationReadParams{ID: id, UserID: userID, TenantID: tenantID})
	return n > 0, err
}

func (s *Service) MarkAllRead(ctx context.Context, userID uuid.UUID) error {
	return gen.New(s.pool).MarkAllNotificationsRead(ctx, userID)
}
