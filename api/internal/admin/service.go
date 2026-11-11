// Package admin implements the super admin seed and the tenant/user management API.
package admin

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"assetku/api/internal/auth"
	"assetku/api/internal/db/gen"
)

// Service serves the super admin API. Sessions, cookies and the login rate
// limiter are reused from auth.Service.
type Service struct {
	pool *pgxpool.Pool
	q    *gen.Queries
	auth *auth.Service
}

func NewService(pool *pgxpool.Pool, a *auth.Service) *Service {
	return &Service{pool: pool, q: gen.New(pool), auth: a}
}

// UpsertSuperAdmin creates the super admin, or when the email (case-insensitive)
// already exists updates its name and password and revokes its sessions.
func UpsertSuperAdmin(ctx context.Context, pool *pgxpool.Pool, email, name, password string) error {
	email, name = strings.TrimSpace(email), strings.TrimSpace(name)
	if _, err := mail.ParseAddress(email); err != nil {
		return errors.New("invalid email")
	}
	if name == "" {
		return errors.New("name is required")
	}
	if p := auth.PasswordProblem(password); p != "" {
		return errors.New("password " + p + " (min 8 characters, max 72 bytes)")
	}
	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	q := gen.New(tx)
	id, err := q.UpsertSuperAdmin(ctx, gen.UpsertSuperAdminParams{Name: name, Email: email, PasswordHash: hash})
	if err != nil {
		return err
	}
	if err := q.DeleteSessionsForSubject(ctx, gen.DeleteSessionsForSubjectParams{SubjectType: auth.SubjectAdmin, SubjectID: id}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func isUniqueViolation(err error) bool {
	var pe *pgconn.PgError
	return errors.As(err, &pe) && pe.Code == "23505"
}

func isNotFound(err error) bool { return errors.Is(err, pgx.ErrNoRows) }
